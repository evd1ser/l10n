package l10n

import (
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func beforeQuery(scope *gorm.DB) {
	if IsLocalizable(scope) {
		quotedTableName := scope.Statement.Table
		quotedPrimaryKey := scope.Statement.Schema.PrioritizedPrimaryField.Name
		hasDeletedAtColumn := scope.Statement.Schema.LookUpField("deleted_at") != nil

		locale, isLocale := getQueryLocale(scope)

		mode, _ := scope.Get("l10n:mode")

		switch mode {
		case "unscoped":
		case "global":
			scope.Where(fmt.Sprintf("%v.language_code = ?", quotedTableName), Global)
		case "locale":
			scope.Where(fmt.Sprintf("%v.language_code = ?", quotedTableName), locale)
		case "reverse":
			if !scope.Statement.Unscoped && hasDeletedAtColumn {
				scope.Where(fmt.Sprintf(
					"(%v.%v NOT IN (SELECT DISTINCT(%v) FROM %v t2 WHERE t2.language_code = ? AND t2.deleted_at IS NULL) AND %v.language_code = ?)", quotedTableName, quotedPrimaryKey, quotedPrimaryKey, quotedTableName, quotedTableName), locale, Global)
			} else {
				scope.Where(fmt.Sprintf("(%v.%v NOT IN (SELECT DISTINCT(%v) FROM %v t2 WHERE t2.language_code = ?) AND %v.language_code = ?)", quotedTableName, quotedPrimaryKey, quotedPrimaryKey, quotedTableName, quotedTableName), locale, Global)
			}
		case "fallback":
			fallthrough
		default:
			if isLocale {
				if !scope.Statement.Unscoped && hasDeletedAtColumn {
					scope.Where(fmt.Sprintf("((%v.%v NOT IN (SELECT DISTINCT(%v) FROM %v t2 WHERE t2.language_code = ? AND t2.deleted_at IS NULL) AND %v.language_code = ?) OR %v.language_code = ?) AND %v.deleted_at IS NULL", quotedTableName, quotedPrimaryKey, quotedPrimaryKey, quotedTableName, quotedTableName, quotedTableName, quotedTableName), locale, Global, locale)
				} else {
					scope.Where(fmt.Sprintf("(%v.%v NOT IN (SELECT DISTINCT(%v) FROM %v t2 WHERE t2.language_code = ?) AND %v.language_code = ?) OR (%v.language_code = ?)", quotedTableName, quotedPrimaryKey, quotedPrimaryKey, quotedTableName, quotedTableName, quotedTableName), locale, Global, locale)
				}

				expr := gorm.Expr(fmt.Sprintf("%v.language_code = ? DESC", quotedTableName), locale)

				scope.Clauses(clause.OrderBy{
					Expression: expr,
				})

				//scope.Order(expr)
			} else {
				scope.Where(fmt.Sprintf("%v.language_code = ?", quotedTableName), Global)
			}
		}
	}
}

func beforeCreate(scope *gorm.DB) {
	if IsLocalizable(scope) {

		if locale, ok := getLocale(scope); ok { // is locale
			syncLocalesCount(scope, locale)
			if isLocaleCreatable(scope) || scope.Statement.Schema.PrioritizedPrimaryField == nil {
				setLocale(scope, locale)
			} else {
				err := fmt.Errorf("the resource %v cannot be created in %v", scope.Statement.Schema.ModelType.Name(), locale)
				scope.Error = err
			}
		} else {
			syncLocalesCount(scope, Global)
			setLocale(scope, Global)
		}
	}
}

func beforeUpdate(scope *gorm.DB) {
	if IsLocalizable(scope) {
		locale, isLocale := getLocale(scope)

		syncLocalesCount(scope, "")

		mode, _ := scope.Get("l10n:mode")

		switch mode {
		case "unscoped":
		default:
			scope.Where(fmt.Sprintf("%v.language_code = ?", scope.Statement.Schema.Table), locale)
			setLocale(scope, locale)
		}

		if isLocale {
			//scope.Omit(syncColumns(scope)...)
		}
	}
}

func afterUpdate(scope *gorm.DB) {
	if scope.Error == nil {
		if IsLocalizable(scope) {
			locale, ok := getLocale(scope)

			if ok {
				if scope.RowsAffected == 0 && scope.Statement.Schema.PrioritizedPrimaryField == nil { //is locale and nothing updated
					var count int64
					var query = fmt.Sprintf("%v.language_code = ? AND %v.%v = ?", scope.Statement.Schema.Table, scope.Statement.Schema.Table, scope.Statement.Schema.PrioritizedPrimaryField.Name)

					// if enabled soft delete, delete soft deleted records
					if scope.Statement.Schema.LookUpField("DeletedAt") != nil {
						scope.Unscoped().Where("deleted_at is not null").Where(query, locale, scope.Statement.Schema.PrioritizedPrimaryField).Delete(scope.Statement.ReflectValue)
					}

					// if no localized records exist, localize it
					if scope.Table(scope.Statement.Schema.Table).Where(query, locale, scope.Statement.Schema.PrioritizedPrimaryField).Count(&count); count == 0 {
						scope.RowsAffected = scope.Create(scope.Statement.ReflectValue).RowsAffected
					}
				}
			}
			if syncColumns := syncColumns(scope); len(syncColumns) > 0 { // is global
				mode, _ := scope.Get("l10n:mode")

				if mode != "unscoped" {

					if scope.RowsAffected > 0 {
						var primaryField = scope.Statement.Schema.PrioritizedPrimaryField
						var syncAttrs = map[string]interface{}{}

						if updateAttrs, ok := scope.InstanceGet("gorm:update_attrs"); ok {
							for key, value := range updateAttrs.(map[string]interface{}) {
								for _, syncColumn := range syncColumns {
									if syncColumn == key {
										syncAttrs[syncColumn] = value
										break
									}
								}
							}
						} else {
							for _, syncColumn := range syncColumns {
								if field := scope.Statement.Schema.LookUpField(syncColumn); field != nil {
									fieldValue, _ := field.ValueOf(scope.Statement.ReflectValue)
									syncAttrs[syncColumn] = fieldValue
								}
							}
						}

						if len(syncAttrs) > 0 {

							db := scope.Session(&gorm.Session{NewDB: true, SkipHooks: true})
							db = db.Table(scope.Statement.Table).Unscoped().Set("l10n:mode", "unscoped").Where("language_code <> ?", locale)

							fieldValue, ok := primaryField.ValueOf(scope.Statement.ReflectValue)

							if !ok {
								db = db.Where(fmt.Sprintf("%v = ?", primaryField.DBName), fieldValue)
							}

							scope.Error = db.UpdateColumns(syncAttrs).Error
						}
					}
				}
			}
		}
	}
}

func beforeDelete(scope *gorm.DB) {
	if IsLocalizable(scope) {
		if locale, ok := getQueryLocale(scope); ok { // is locale
			scope.Where(fmt.Sprintf("%v.language_code = ?", scope.Statement.Schema.Table), locale)
		}
	}
}

// RegisterCallbacks register callback into GORM DB
func RegisterCallbacks(db *gorm.DB) {
	callback := db.Callback()

	if callback.Create().Get("l10n:before_create") == nil {
		callback.Create().Before("gorm:before_create").Register("l10n:before_create", beforeCreate)
	}

	if callback.Create().Get("l10n:after_create") == nil {
		callback.Create().Before("gorm:after_create").Register("l10n:after_create", afterUpdate)
	}

	if callback.Update().Get("l10n:before_update") == nil {
		callback.Update().Before("gorm:before_update").Register("l10n:before_update", beforeUpdate)
	}
	if callback.Update().Get("l10n:after_update") == nil {
		callback.Update().After("gorm:after_update").Register("l10n:after_update", afterUpdate)
	}

	if callback.Delete().Get("l10n:before_delete") == nil {
		callback.Delete().Before("gorm:before_delete").Register("l10n:before_delete", beforeDelete)
	}

	if callback.Row().Get("l10n:before_query") == nil {
		callback.Row().Before("gorm:row_query").Register("l10n:before_query", beforeQuery)
	}
	if callback.Query().Get("l10n:before_query") == nil {
		callback.Query().Before("gorm:query").Register("l10n:before_query", beforeQuery)
	}
}
