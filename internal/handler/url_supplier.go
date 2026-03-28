package handler

import (
	"fmt"
)

// URLSupplier формирует URL на основе шаблона с параметрами.
// Шаблон должен содержать форматные спецификаторы (например, %s),
// которые будут заменены переданными значениями.
type URLSupplier struct {
	template string
}

// NewURLSupplier создает новый экземпляр URLSupplier с заданным шаблоном URL.
func NewURLSupplier(template string) *URLSupplier {
	return &URLSupplier{template: template}
}

// WithPathParams подставляет в шаблон параметры в виде плейсхолдеров (например, :1, :2).
// Используется для генерации URL с именованными параметрами.
func (s *URLSupplier) WithPathParams(params ...PathKey) string {
	templatedParams := make([]string, len(params))
	for i, param := range params {
		templatedParams[i] = fmt.Sprintf(":%v", param)
	}
	return fmt.Sprintf(s.template, s.toInterfaceSlice(templatedParams)...)
}

// WithValues подставляет в шаблон конкретные значения параметров.
// Значения должны соответствовать количеству и порядку форматных спецификаторов в шаблоне.
func (s *URLSupplier) WithValues(values ...string) string {
	return fmt.Sprintf(s.template, s.toInterfaceSlice(values)...)
}

// toInterfaceSlice преобразует срез строк в срез интерфейсов.
func (s *URLSupplier) toInterfaceSlice(ss []string) []interface{} {
	res := make([]interface{}, len(ss))
	for i, v := range ss {
		res[i] = v
	}
	return res
}
