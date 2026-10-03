import { messageFor } from "../../problem/problem";

// Text for a refused link, by the API's problem code (never the HTTP status: all of these are 409).
// EVENT_ALREADY_LINKED and INVOICE_ALREADY_PAID are accepted as aliases of the codes the server sends today.
export const linkErrorMessage = (e: unknown) =>
  messageFor(e, {
    LINK_AMOUNT_MISMATCH: "money.amountMismatch",
    EVENT_NOT_LINKABLE: "money.alreadyLinked",
    EVENT_ALREADY_LINKED: "money.alreadyLinked",
    INVOICE_NOT_OPEN: "money.invoiceNotOpen",
    INVOICE_ALREADY_PAID: "money.invoiceNotOpen",
  });
