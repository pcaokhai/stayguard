package payment

import (
	"errors"
	"fmt"
)

var ErrQRInput = errors.New("invalid QR input")

func tlv(id, val string) string { return fmt.Sprintf("%s%02d%s", id, len(val), val) }

// crc16 is CRC-16/CCITT-FALSE (poly 0x1021, init 0xFFFF), the checksum EMVCo QR codes end with.
func crc16(s string) uint16 {
	crc := uint16(0xFFFF)
	for i := 0; i < len(s); i++ {
		crc ^= uint16(s[i]) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// QRPayload builds the dynamic VietQR (NAPAS 247) string that pays accountNo at bank bin for exactly
// amount VND, with note as the transfer content. The caller passes the tenant's own account.
func QRPayload(bin, accountNo string, amount int64, note string) (string, error) {
	if bin == "" || accountNo == "" || amount <= 0 || note == "" || len(accountNo) > 19 || len(note) > 25 {
		return "", ErrQRInput
	}
	member := tlv("00", bin) + tlv("01", accountNo)
	body := tlv("00", "01") + tlv("01", "12") +
		tlv("38", tlv("00", "A000000727")+tlv("01", member)+tlv("02", "QRIBFTTA")) +
		tlv("53", "704") + tlv("54", fmt.Sprint(amount)) + tlv("58", "VN") +
		tlv("62", tlv("08", note)) + "6304"
	return fmt.Sprintf("%s%04X", body, crc16(body)), nil
}
