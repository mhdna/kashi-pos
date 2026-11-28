package main

import (
	"fmt"
	"strings"
)

// The till's screen language, "en" (the default) or "fr". Set from central
// per branch or from Settings on the till; kashi's own screen language is
// unrelated. Stored as one flag, on until set, so a till never set is English.
const (
	tillLanguageEnglishFlag = "till_language_english"
	tillLanguageFrench      = "fr"
	tillLanguageEnglish     = "en"
)

func (a *App) tillLanguage() string {
	if a.flagEnabledDefaultOn(a.ctx, tillLanguageEnglishFlag) {
		return tillLanguageEnglish
	}
	return tillLanguageFrench
}

// SetTillLanguage changes the screen language of this till.
func (a *App) SetTillLanguage(language string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	language = strings.ToLower(strings.TrimSpace(language))
	if language != tillLanguageFrench && language != tillLanguageEnglish {
		return fmt.Errorf("The language must be French or English")
	}
	if err := a.setFeatureFlagBool(a.ctx, tillLanguageEnglishFlag, language == tillLanguageEnglish); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}
