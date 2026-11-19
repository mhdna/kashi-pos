package main

import (
	_ "embed"
	"strings"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/goregular"
)

// Every font a receipt line (or a part, or a banner) can be set in, from
// central's Receipt Designer: the looks' own fonts and extra ones, all under
// open licences (assets/fonts/themes/LICENSES.txt, assets/fonts/extra/LICENSES.txt).

//go:embed assets/fonts/extra/AbrilFatface-Regular.ttf
var abrilFatfaceRegular []byte

//go:embed assets/fonts/extra/Anton-Regular.ttf
var antonRegular []byte

//go:embed assets/fonts/extra/ArchivoBlack-Regular.ttf
var archivoBlackRegular []byte

//go:embed assets/fonts/extra/BebasNeue-Regular.ttf
var bebasNeueRegular []byte

//go:embed assets/fonts/extra/DancingScript-Regular.ttf
var dancingScriptRegular []byte

//go:embed assets/fonts/extra/DancingScript-Bold.ttf
var dancingScriptBold []byte

//go:embed assets/fonts/extra/Inter-Regular.ttf
var interRegular []byte

//go:embed assets/fonts/extra/Inter-Bold.ttf
var interBold []byte

//go:embed assets/fonts/extra/Lobster-Regular.ttf
var lobsterRegular []byte

//go:embed assets/fonts/extra/Merriweather-Regular.ttf
var merriweatherRegular []byte

//go:embed assets/fonts/extra/Merriweather-Bold.ttf
var merriweatherBold []byte

//go:embed assets/fonts/extra/Montserrat-Regular.ttf
var montserratRegular []byte

//go:embed assets/fonts/extra/Montserrat-Bold.ttf
var montserratBold []byte

//go:embed assets/fonts/extra/Oswald-Regular.ttf
var oswaldRegular []byte

//go:embed assets/fonts/extra/Oswald-Bold.ttf
var oswaldBold []byte

//go:embed assets/fonts/extra/Pacifico-Regular.ttf
var pacificoRegular []byte

//go:embed assets/fonts/extra/Poppins-Regular.ttf
var poppinsRegular []byte

//go:embed assets/fonts/extra/Poppins-Bold.ttf
var poppinsBold []byte

//go:embed assets/fonts/extra/Righteous-Regular.ttf
var righteousRegular []byte

//go:embed assets/fonts/extra/RobotoMono-Regular.ttf
var robotoMonoRegular []byte

//go:embed assets/fonts/extra/RobotoMono-Bold.ttf
var robotoMonoBold []byte

//go:embed assets/fonts/extra/SpaceMono-Regular.ttf
var spaceMonoRegular []byte

//go:embed assets/fonts/extra/SpaceMono-Bold.ttf
var spaceMonoBold []byte

//go:embed assets/fonts/extra/SpecialElite-Regular.ttf
var specialEliteRegular []byte

//go:embed assets/fonts/arabic/Vazirmatn-Regular.ttf
var vazirmatnRegular []byte

//go:embed assets/fonts/arabic/Vazirmatn-Bold.ttf
var vazirmatnBold []byte

//go:embed assets/fonts/arabic/NotoNaskhArabic-Regular.ttf
var notoNaskhArabicRegular []byte

//go:embed assets/fonts/arabic/NotoNaskhArabic-Bold.ttf
var notoNaskhArabicBold []byte

// receiptFont is one font a line can be set in.
type receiptFont struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	// Kind groups the list: sans, serif, mono, display or script.
	Kind string `json:"kind"`
	// Scale evens out faces that read small or large at one size.
	Scale         float64 `json:"-"`
	Regular, Bold []byte  `json:"-"`
}

// receiptFonts is the list, in the designer's order. A font with one weight
// draws bold and regular alike.
var receiptFonts = []receiptFont{
	{Key: "go-mono", Label: "Go Mono", Kind: "mono", Scale: 1, Regular: gomono.TTF, Bold: gomonobold.TTF},
	{Key: "go-sans", Label: "Go Sans", Kind: "sans", Scale: 1, Regular: goregular.TTF, Bold: gobold.TTF},
	{Key: "cascadia-mono", Label: "Cascadia Mono", Kind: "mono", Scale: 1, Regular: cascadiaMonoRegular, Bold: cascadiaMonoBold},
	{Key: "plex-mono", Label: "IBM Plex Mono", Kind: "mono", Scale: 0.9, Regular: plexMonoRegular, Bold: plexMonoBold},
	{Key: "courier-prime", Label: "Courier Prime", Kind: "mono", Scale: 1, Regular: courierPrimeRegular, Bold: courierPrimeBold},
	{Key: "roboto-mono", Label: "Roboto Mono", Kind: "mono", Scale: 0.95, Regular: robotoMonoRegular, Bold: robotoMonoBold},
	{Key: "space-mono", Label: "Space Mono", Kind: "mono", Scale: 0.92, Regular: spaceMonoRegular, Bold: spaceMonoBold},
	{Key: "special-elite", Label: "Special Elite (typewriter)", Kind: "mono", Scale: 1, Regular: specialEliteRegular, Bold: specialEliteRegular},
	{Key: "manrope", Label: "Manrope", Kind: "sans", Scale: 1, Regular: manropeMedium, Bold: manropeExtraBold},
	{Key: "jost", Label: "Jost", Kind: "sans", Scale: 1, Regular: jostRegular, Bold: jostSemiBold},
	{Key: "inter", Label: "Inter", Kind: "sans", Scale: 0.97, Regular: interRegular, Bold: interBold},
	{Key: "poppins", Label: "Poppins", Kind: "sans", Scale: 0.95, Regular: poppinsRegular, Bold: poppinsBold},
	{Key: "montserrat", Label: "Montserrat", Kind: "sans", Scale: 0.95, Regular: montserratRegular, Bold: montserratBold},
	{Key: "belleza", Label: "Belleza", Kind: "sans", Scale: 1.05, Regular: bellezaFont, Bold: bellezaFont},
	{Key: "lora", Label: "Lora", Kind: "serif", Scale: 1, Regular: loraRegular, Bold: loraBold},
	{Key: "merriweather", Label: "Merriweather", Kind: "serif", Scale: 0.92, Regular: merriweatherRegular, Bold: merriweatherBold},
	{Key: "cormorant", Label: "Cormorant Garamond", Kind: "serif", Scale: 1.25, Regular: cormorantMedium, Bold: cormorantBold},
	{Key: "playfair", Label: "Playfair Display", Kind: "serif", Scale: 1, Regular: playfairExtraBold, Bold: playfairExtraBold},
	{Key: "oswald", Label: "Oswald", Kind: "display", Scale: 1.05, Regular: oswaldRegular, Bold: oswaldBold},
	{Key: "bebas-neue", Label: "Bebas Neue", Kind: "display", Scale: 1.2, Regular: bebasNeueRegular, Bold: bebasNeueRegular},
	{Key: "anton", Label: "Anton", Kind: "display", Scale: 1.05, Regular: antonRegular, Bold: antonRegular},
	{Key: "archivo-black", Label: "Archivo Black", Kind: "display", Scale: 0.92, Regular: archivoBlackRegular, Bold: archivoBlackRegular},
	{Key: "abril-fatface", Label: "Abril Fatface", Kind: "display", Scale: 1, Regular: abrilFatfaceRegular, Bold: abrilFatfaceRegular},
	{Key: "righteous", Label: "Righteous", Kind: "display", Scale: 1, Regular: righteousRegular, Bold: righteousRegular},
	{Key: "lobster", Label: "Lobster", Kind: "script", Scale: 1.05, Regular: lobsterRegular, Bold: lobsterRegular},
	{Key: "pacifico", Label: "Pacifico", Kind: "script", Scale: 0.92, Regular: pacificoRegular, Bold: pacificoRegular},
	{Key: "dancing-script", Label: "Dancing Script", Kind: "script", Scale: 1.2, Regular: dancingScriptRegular, Bold: dancingScriptBold},
	// Arabic and Latin both: Arabic lines are drawn in one of these.
	{Key: "vazirmatn", Label: "Vazirmatn (Arabic)", Kind: "arabic", Scale: 1, Regular: vazirmatnRegular, Bold: vazirmatnBold},
	{Key: "noto-naskh-arabic", Label: "Noto Naskh (Arabic)", Kind: "arabic", Scale: 1.05, Regular: notoNaskhArabicRegular, Bold: notoNaskhArabicBold},
}

// arabicFontDefault is the font Arabic lines are drawn in unless the line's
// own font is an Arabic one.
const arabicFontDefault = "vazirmatn"

func receiptFontByKey(key string) (receiptFont, bool) {
	for _, f := range receiptFonts {
		if f.Key == key {
			return f, true
		}
	}
	return receiptFont{}, false
}

// receiptFontKeyOf names the font a file belongs to, for the designer to
// set its typing box in the same font; "" for one not on the list.
func receiptFontKeyOf(data []byte) (key string, bold bool) {
	if len(data) == 0 {
		return "", false
	}
	for _, f := range receiptFonts {
		if len(f.Regular) > 0 && &f.Regular[0] == &data[0] {
			return f.Key, false
		}
		if len(f.Bold) > 0 && &f.Bold[0] == &data[0] {
			return f.Key, true
		}
	}
	return "", false
}

// validReceiptFont keeps a known font's key, else "".
func validReceiptFont(key string) string {
	key = strings.TrimSpace(key)
	if _, ok := receiptFontByKey(key); ok {
		return key
	}
	return ""
}

// useFont sets the canvas's text in a font from the list until the returned
// func runs ("" is the look's own).
func (c *receiptCanvas) useFont(key string) func() {
	saved := c.fontKey
	c.fontKey = key
	return func() { c.fontKey = saved }
}

// ReceiptFontFile is a font's file, for the designer to show lines in it.
func ReceiptFontFile(key string, bold bool) ([]byte, bool) {
	f, ok := receiptFontByKey(key)
	if !ok {
		return nil, false
	}
	if bold {
		return f.Bold, true
	}
	return f.Regular, true
}

// Every line that can change its size can change its font: both go through
// the same drawing (line, lineWrapped, row, the meta lines, the socials title).
func init() {
	for i := range receiptElements {
		// Column titles and TOTAL are words on a row: their font follows them
		// (pendingFont).
		if receiptElements[i].Can&canSize != 0 || strings.HasPrefix(receiptElements[i].Key, "col_") || receiptElements[i].Key == "total" {
			receiptElements[i].Can |= canFont
		}
	}
}
