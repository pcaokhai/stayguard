package httpadapter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// maxPhotoBytes is the largest photo (docs/15 rule 24); the body cap of the upload route adds room for the multipart framing.
const (
	maxPhotoBytes     = 5 << 20
	photoRouteMaxBody = maxPhotoBytes + 256<<10
)

// GuestIDService is the guest ID surface the handlers need.
type GuestIDService interface {
	SetNumber(ctx context.Context, c app.Caller, stayID, idNumber string, consent bool) (app.GuestIDIndicators, error)
	UploadPhoto(ctx context.Context, c app.Caller, stayID, side string, raw []byte, consent bool) (app.GuestIDIndicators, error)
	Record(ctx context.Context, c app.Caller, stayID string) (app.GuestIDRecordView, error)
	Reveal(ctx context.Context, c app.Caller, stayID string) (string, error)
	Photo(ctx context.Context, c app.Caller, stayID, side string, download bool) ([]byte, error)
	DeletePhoto(ctx context.Context, c app.Caller, stayID, side string) error
	DeleteNumber(ctx context.Context, c app.Caller, stayID string) error
}

// WithGuestIDs adds the guest ID use cases; the router always sets them.
func (s Server) WithGuestIDs(g GuestIDService) Server { s.guestIDs = g; return s }

func toIndicators(i app.GuestIDIndicators) gen.GuestIdIndicators {
	return gen.GuestIdIndicators{HasIdNumber: i.HasIDNumber, HasFrontPhoto: i.HasFrontPhoto, HasBackPhoto: i.HasBackPhoto}
}

func (s Server) SetGuestIdNumber(ctx context.Context, req gen.SetGuestIdNumberRequestObject) (gen.SetGuestIdNumberResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil || req.Body.IdNumber == nil {
		return nil, &app.ValidationError{Field: "idNumber", Reason: "required"}
	}
	ind, err := s.guestIDs.SetNumber(ctx, c, req.StayId, *req.Body.IdNumber, bool(req.Body.Consent))
	if err != nil {
		return nil, err
	}
	return gen.SetGuestIdNumber200JSONResponse(toIndicators(ind)), nil
}

// readPhotoForm takes the file and the optional consent flag out of the multipart body. The file is read with a hard
// cap so an endless upload cannot fill memory; it is never logged or echoed.
func readPhotoForm(mr *multipart.Reader) (file []byte, consent bool, err error) {
	if mr == nil {
		return nil, false, &app.ValidationError{Field: "file", Reason: "required"}
	}
	for {
		part, perr := mr.NextPart()
		if errors.Is(perr, io.EOF) {
			break
		}
		if perr != nil {
			return nil, false, bodyError(perr)
		}
		switch part.FormName() {
		case "file":
			b, rerr := io.ReadAll(io.LimitReader(part, maxPhotoBytes+1))
			if rerr != nil {
				return nil, false, bodyError(rerr)
			}
			if len(b) > maxPhotoBytes {
				return nil, false, app.ErrPhotoTooLarge
			}
			file = b
		case "consent":
			b, _ := io.ReadAll(io.LimitReader(part, 8))
			consent = strings.EqualFold(strings.TrimSpace(string(b)), "true")
		}
	}
	if file == nil {
		return nil, false, &app.ValidationError{Field: "file", Reason: "required"}
	}
	return file, consent, nil
}

// bodyError says too large when the body cap fired and "cannot read" otherwise.
func bodyError(err error) error {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		return app.ErrPhotoTooLarge
	}
	return &app.ValidationError{Field: "file", Reason: "cannot be read"}
}

func (s Server) UploadGuestIdPhoto(ctx context.Context, req gen.UploadGuestIdPhotoRequestObject) (gen.UploadGuestIdPhotoResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	file, consent, err := readPhotoForm(req.Body)
	if err != nil {
		return nil, err
	}
	ind, err := s.guestIDs.UploadPhoto(ctx, c, req.StayId, string(req.Side), file, consent)
	if err != nil {
		return nil, err
	}
	return gen.UploadGuestIdPhoto200JSONResponse(toIndicators(ind)), nil
}

func (s Server) GetGuestIdRecord(ctx context.Context, req gen.GetGuestIdRecordRequestObject) (gen.GetGuestIdRecordResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.guestIDs.Record(ctx, c, req.StayId)
	if err != nil {
		return nil, err
	}
	out := gen.GuestIdRecord{Indicators: toIndicators(v.Indicators), IdNumberMasked: v.IDNumberMasked, ConsentAt: v.ConsentAt}
	if v.DeleteAfter != nil {
		out.DeleteAfter = &openapi_types.Date{Time: *v.DeleteAfter}
	}
	for _, p := range v.Photos {
		bytes := p.Bytes
		meta := &gen.IdPhotoMeta{Side: gen.IdPhotoMetaSide(p.Side), UploadedAt: p.UploadedAt, UploadedBy: p.UploadedBy, Bytes: &bytes}
		if p.Side == "FRONT" {
			out.Front = meta
		} else {
			out.Back = meta
		}
	}
	return gen.GetGuestIdRecord200JSONResponse(out), nil
}

func (s Server) RevealGuestIdNumber(ctx context.Context, req gen.RevealGuestIdNumberRequestObject) (gen.RevealGuestIdNumberResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	n, err := s.guestIDs.Reveal(ctx, c, req.StayId)
	if err != nil {
		return nil, err
	}
	return gen.RevealGuestIdNumber200JSONResponse{IdNumber: n}, nil
}

// photoResponse streams a JPEG with the headers guest ID data always gets: never cached, never sniffed.
type photoResponse struct {
	img      []byte
	filename string // set for a download
}

func (p photoResponse) VisitGetGuestIdPhotoResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", fmt.Sprint(len(p.img)))
	if p.filename != "" {
		w.Header().Set("Content-Disposition", `attachment; filename="`+p.filename+`"`)
	}
	w.WriteHeader(http.StatusOK)
	_, err := io.Copy(w, bytes.NewReader(p.img))
	return err
}

func (s Server) GetGuestIdPhoto(ctx context.Context, req gen.GetGuestIdPhotoRequestObject) (gen.GetGuestIdPhotoResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	download := req.Params.Download != nil && *req.Params.Download
	img, err := s.guestIDs.Photo(ctx, c, req.StayId, string(req.Side), download)
	if err != nil {
		return nil, err
	}
	out := photoResponse{img: img}
	if download {
		out.filename = "guest-id-" + strings.ToLower(string(req.Side)) + ".jpg"
	}
	return out, nil
}

func (s Server) DeleteGuestIdPhoto(ctx context.Context, req gen.DeleteGuestIdPhotoRequestObject) (gen.DeleteGuestIdPhotoResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.guestIDs.DeletePhoto(ctx, c, req.StayId, string(req.Side)); err != nil {
		return nil, err
	}
	return gen.DeleteGuestIdPhoto204Response{}, nil
}

func (s Server) DeleteGuestIdNumber(ctx context.Context, req gen.DeleteGuestIdNumberRequestObject) (gen.DeleteGuestIdNumberResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.guestIDs.DeleteNumber(ctx, c, req.StayId); err != nil {
		return nil, err
	}
	return gen.DeleteGuestIdNumber204Response{}, nil
}
