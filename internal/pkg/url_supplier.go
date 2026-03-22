package pkg

import "fmt"

type URLSupplier struct {
	template string
}

func NewURLSupplier(template string) *URLSupplier {
	return &URLSupplier{template: template}
}

func (s *URLSupplier) WithTemplateParams(params ...int) string {
	templatedParams := make([]string, len(params))
	for i, param := range params {
		templatedParams[i] = fmt.Sprintf(":%v", param)
	}
	return fmt.Sprintf(s.template, s.toInterfaceSlice(templatedParams)...)
}

func (s *URLSupplier) WithValues(values ...string) string {
	return fmt.Sprintf(s.template, s.toInterfaceSlice(values)...)
}

func (s *URLSupplier) toInterfaceSlice(ss []string) []interface{} {
	res := make([]interface{}, len(ss))
	for i, v := range ss {
		res[i] = v
	}
	return res
}
