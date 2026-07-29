package main

import (
	"flag"
	"fmt"
)

func runReceiptPreview(args []string) error {
	flags := flag.NewFlagSet("receipt-preview", flag.ContinueOnError)
	output := flags.String("output", "receipt-preview.pdf", "path for the generated PDF")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	if *output == "" {
		return fmt.Errorf("output path cannot be empty")
	}

	data := printReceiptData{
		Items: []invoiceItem{
			{Name: "Classic Cotton T-Shirtfjjjffffffffffffffffffffffffffffffffffffffffffffffffffffffff", Barcode: "100000000004", Qty: 2, Price: 18.00},
			{Name: "Canvas Everyday Sneakers", Barcode: "100000000005", Qty: 1, Price: 42.50, Discount: 10},
		},
		StoreName:       "Kashi Market",
		Branch:          "Downtown Branch",
		BranchPhone:     "+961 1 234 567",
		Description:     "Everyday essentials and thoughtful goods",
		CounterNumber:   "04",
		Salesperson:     "Sarah Demo",
		Customer:        "Walk-in Customer",
		ReturnPolicy:    "Exchanges accepted within 3 days.",
		Website:         "kashi.example",
		SocialPlatforms: []string{"facebook", "tiktok", "instagram", "threads", "youtube"},
		SocialHandles: map[string]string{
			"facebook":  "@kashimarket",
			"tiktok":    "@kashimarket",
			"instagram": "@kashimarket",
			"threads":   "@kashimarket",
			"youtube":   "Kashi Market",
		},
		Font:            "Belleza",
		BodyFont:        "CascadiaMono",
		TitleSize:       21,
		BodySize:        11,
		ReceiptNo:       "INV-PREVIEW-0001",
		GrandTotalCents: 7900,
		Payments: []receiptPayment{{
			AccountName:  "Cash",
			CurrencyCode: "USD",
			AmountCents:  7900,
		}},
		ExchangeRate:  90000,
		RoundingMode:  "nearest",
		ReceiptWidth:  80,
		ReceiptHeight: 297,
	}

	pdf := buildReceiptPDF(data)
	if err := pdf.OutputFileAndClose(*output); err != nil {
		return fmt.Errorf("write PDF: %w", err)
	}
	fmt.Printf("receipt preview written to %s\n", *output)
	return nil
}
