package payment

// banks maps the VietQR bank identification number (BIN) to a short name for the owner's account list.
// ponytail: the common banks only; any other BIN shows as "Bank <BIN>" and still works in the QR.
var banks = map[string]string{
	"970436": "Vietcombank", "970415": "VietinBank", "970418": "BIDV", "970405": "Agribank", "970422": "MB",
	"970407": "Techcombank", "970416": "ACB", "970432": "VPBank", "970423": "TPBank", "970403": "Sacombank",
	"970437": "HDBank", "970441": "VIB", "970443": "SHB", "970426": "MSB", "970431": "Eximbank",
	"970448": "OCB", "970440": "SeABank", "970449": "LPBank", "970429": "SCB", "970425": "ABBANK", "970409": "BacABank",
}

// BankName is the short name of a bank by BIN, or "Bank <BIN>" when it is not in the list.
func BankName(bin string) string {
	if n, ok := banks[bin]; ok {
		return n
	}
	return "Bank " + bin
}
