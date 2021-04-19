package l10n

import (
	"fmt"
	"github.com/qor/qor/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// IsLocalizable return model is localizable or not
func IsLocalizable(db *gorm.DB) (IsLocalizable bool) {
	model := db.Statement.Model
	if model == nil {
		return false
	}

	_, IsLocalizable = model.(l10nInterface)

	return
}

type localeCreatableInterface interface {
	CreatableFromLocale()
}

type localeCreatableInterface2 interface {
	LocaleCreatable()
}

func isLocaleCreatable(db *gorm.DB) (ok bool) {
	model := db.Statement.Model

	if _, ok = model.(localeCreatableInterface); ok {
		return
	}
	_, ok = model.(localeCreatableInterface2)
	return
}

func setLocale(db *gorm.DB, locale string) {
	sch := db.Statement.Schema

	for _, field := range sch.Fields {
		if field.Name == "LanguageCode" {
			field.Set(db.Statement.ReflectValue, locale)
		}
	}
}

func getQueryLocale(scope *gorm.DB) (locale string, isLocale bool) {
	if str, ok := scope.Get("l10n:locale"); ok {
		if locale, ok := str.(string); ok && locale != "" {

			return locale, locale != Global
		}
	}
	return Global, false
}

func getLocale(scope *gorm.DB) (locale string, isLocale bool) {
	if str, ok := scope.Get("l10n:localize_to"); ok {
		if locale, ok := str.(string); ok && locale != "" {
			return locale, locale != Global
		}
	}

	return getQueryLocale(scope)
}

func isSyncField(field *schema.Field) bool {
	_, ok := utils.ParseTagOption(field.Tag.Get("l10n"))["SYNC"]
	if ok {
		return true
	}
	return false
}

func syncColumns(db *gorm.DB) (columns []string) {
	sch := db.Statement.Schema

	for _, field := range sch.Fields {
		if isSyncField(field) {
			columns = append(columns, field.DBName)
		}
	}

	return
}

func syncLocalesCount(scope *gorm.DB, locale string) {
	sch := scope.Statement.Schema
	model := scope.Statement.Model

	if model == nil {
		return
	}

	_, hasAvailableLocale := model.(l10nAvailable)

	if !hasAvailableLocale {
		return
	}

	pField := scope.Statement.Schema.PrioritizedPrimaryField
	languageCodes := make([]string, 0, 2)

	if fieldValue, isZero := pField.ValueOf(scope.Statement.ReflectValue); !isZero {
		db := scope.Session(&gorm.Session{NewDB: true})
		db = db.Model(model).Set("l10n:mode", "unscoped").Where(fmt.Sprintf("%v = ?", pField.Name), fieldValue).Pluck("language_code", &languageCodes)
	}

	if locale != "" {
		languageCodes = append(languageCodes, locale)
	}

	for _, field := range sch.Fields {
		if field.Name == "LanguageAvailableCode" {
			field.Set(scope.Statement.ReflectValue, languageCodes)
		}
	}
}
