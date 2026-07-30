package admind

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

const attendanceLeaveRequestMaximumMultipartBytes = 55 << 20

func readAttendanceLeaveRequestMultipartInput(
	responseWriter http.ResponseWriter,
	request *http.Request,
) (attendanceLeaveRequestInput, []attendanceLeaveRequestAttachmentUpload, error) {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, attendanceLeaveRequestMaximumMultipartBytes)
	if errorValue := request.ParseMultipartForm(1 << 20); errorValue != nil {
		return attendanceLeaveRequestInput{}, nil, attendanceLeaveRequestMultipartError(errorValue)
	}
	if request.MultipartForm != nil {
		defer func() {
			if errorValue := request.MultipartForm.RemoveAll(); errorValue != nil {
				slog.WarnContext(
					request.Context(),
					"attendance leave multipart cleanup failed",
					"error",
					errorValue,
				)
			}
		}()
		for fieldName := range request.MultipartForm.File {
			if fieldName != "attachments" {
				return attendanceLeaveRequestInput{}, nil, attendanceLeaveInvalidAttachmentError(
					fmt.Errorf("unsupported multipart file field %q", fieldName),
				)
			}
		}
		for fieldName := range request.MultipartForm.Value {
			if fieldName != "request" {
				return attendanceLeaveRequestInput{}, nil, attendanceLeaveInvalidInputErrorf(
					"unsupported multipart value field %q",
					fieldName,
				)
			}
		}
	}
	requestValues := request.MultipartForm.Value["request"]
	if len(requestValues) != 1 || strings.TrimSpace(requestValues[0]) == "" {
		return attendanceLeaveRequestInput{}, nil, attendanceLeaveInvalidInputErrorf("multipart request field is required")
	}
	var input attendanceLeaveRequestInput
	decoder := json.NewDecoder(strings.NewReader(requestValues[0]))
	decoder.DisallowUnknownFields()
	if errorValue := decoder.Decode(&input); errorValue != nil {
		return attendanceLeaveRequestInput{}, nil, attendanceLeaveInvalidInputError(errorValue)
	}
	if errorValue := ensureAttendanceLeaveRequestJSONEnd(decoder); errorValue != nil {
		return attendanceLeaveRequestInput{}, nil, attendanceLeaveInvalidInputError(errorValue)
	}
	fileHeaders := request.MultipartForm.File["attachments"]
	if len(fileHeaders) > attendanceLeaveRequestMaximumAttachmentCount {
		return attendanceLeaveRequestInput{}, nil, attendanceLeaveInvalidAttachmentError(
			fmt.Errorf(
				"leave request supports at most %d attachments",
				attendanceLeaveRequestMaximumAttachmentCount,
			),
		)
	}
	uploads := make([]attendanceLeaveRequestAttachmentUpload, 0, len(fileHeaders))
	for _, fileHeader := range fileHeaders {
		file, errorValue := fileHeader.Open()
		if errorValue != nil {
			return attendanceLeaveRequestInput{}, nil, attendanceLeaveInvalidAttachmentError(errorValue)
		}
		content, readError := io.ReadAll(io.LimitReader(file, attendanceLeaveRequestMaximumAttachmentBytes+1))
		closeError := file.Close()
		if readError != nil {
			return attendanceLeaveRequestInput{}, nil, attendanceLeaveInvalidAttachmentError(readError)
		}
		if closeError != nil {
			return attendanceLeaveRequestInput{}, nil, attendanceLeaveInvalidAttachmentError(closeError)
		}
		upload, errorValue := normalizeAttendanceLeaveRequestAttachmentUpload(fileHeader.Filename, content)
		if errorValue != nil {
			return attendanceLeaveRequestInput{}, nil, attendanceLeaveInvalidAttachmentError(errorValue)
		}
		uploads = append(uploads, upload)
	}
	return input, uploads, nil
}

func attendanceLeaveRequestMultipartError(errorValue error) error {
	var maximumBytesError *http.MaxBytesError
	if errors.As(errorValue, &maximumBytesError) {
		return fmt.Errorf(
			"%w: payload exceeds %d bytes",
			errAttendanceLeaveRequestTooLarge,
			maximumBytesError.Limit,
		)
	}
	return attendanceLeaveInvalidInputError(errorValue)
}

func ensureAttendanceLeaveRequestJSONEnd(decoder *json.Decoder) error {
	var extra any
	errorValue := decoder.Decode(&extra)
	if errors.Is(errorValue, io.EOF) {
		return nil
	}
	if errorValue != nil {
		return errorValue
	}
	return fmt.Errorf("request JSON must contain exactly one object")
}
