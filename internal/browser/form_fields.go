package browser

import (
	"io"
	"mime"
	"mime/multipart"
	"net/url"
	"strings"
)

type FormField struct {
	Name     string
	Value    string
	Filename string
}

func ContentTypeOf(headers map[string]string) string {
	for name, value := range headers {
		if strings.EqualFold(name, "Content-Type") {
			return value
		}
	}
	return ""
}

func FormFields(contentType string, body string) []FormField {
	mediaType, parameters, errorValue := mime.ParseMediaType(contentType)
	if errorValue != nil || body == "" {
		return nil
	}
	switch mediaType {
	case "application/x-www-form-urlencoded":
		return urlEncodedFields(body)
	case "multipart/form-data":
		return multipartFields(body, parameters["boundary"])
	}
	return nil
}

func urlEncodedFields(body string) []FormField {
	fields := []FormField{}
	for _, field := range strings.Split(body, "&") {
		name, value, _ := strings.Cut(field, "=")
		fields = append(fields, FormField{Name: formComponent(name), Value: formComponent(value)})
	}
	return fields
}

func formComponent(component string) string {
	decoded, errorValue := url.QueryUnescape(component)
	if errorValue != nil {
		return component
	}
	return decoded
}

func multipartFields(body string, boundary string) []FormField {
	if boundary == "" {
		return nil
	}
	reader := multipart.NewReader(strings.NewReader(body), boundary)
	fields := []FormField{}
	for {
		part, errorValue := reader.NextPart()
		if errorValue != nil {
			return fields
		}
		content, readError := io.ReadAll(part)
		if readError != nil {
			return fields
		}
		fields = append(fields, FormField{Name: part.FormName(), Value: string(content), Filename: part.FileName()})
	}
}
