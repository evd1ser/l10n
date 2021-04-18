package l10n

import "github.com/lib/pq"

// Global global language
var Global = "en-US"

type l10nInterface interface {
	IsGlobal() bool
	SetLocale(locale string)
}

// Locale embed this struct into GROM-backend models to enable localization feature for your model
type Locale struct {
	LanguageCode string `sql:"size:20" gorm:"primary_key"`
}

// IsGlobal return if current locale is global
func (l Locale) IsGlobal() bool {
	return l.LanguageCode == Global
}

// SetLocale set model's locale
func (l *Locale) SetLocale(locale string) {
	l.LanguageCode = locale
}

// LocaleCreatable if you embed it into your model, it will make the resource be creatable from locales, by default, you can only create it from global
type LocaleCreatable struct {
	Locale
}

// CreatableFromLocale a method to allow your mod=el be creatable from locales
func (LocaleCreatable) CreatableFromLocale() {}

type availableLocalesInterface interface {
	AvailableLocales() []string
}

type viewableLocalesInterface interface {
	ViewableLocales() []string
}

type editableLocalesInterface interface {
	EditableLocales() []string
}

// LocalizeActionArgument localize action's argument
type LocalizeActionArgument struct {
	From string
	To   []string
}

type LocaleCodes struct {
	LanguageAvailableCode pq.StringArray `gorm:"type:varchar(64)[]" json:"LanguageAvailableCode" l10n:"sync"`
}

type l10nAvailable interface {
	getAllLocales() []string
}

func (lc *LocaleCodes) getAllLocales() []string {
	return  lc.LanguageAvailableCode
}
