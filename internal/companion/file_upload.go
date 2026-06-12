package companion

import (
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"strconv"
)

type DeviceFileUploader struct {
	DeviceClient DeviceClient
}

type fileUploadCreateResponse struct {
	UploadID  string `json:"uploadID"`
	ChunkSize int64  `json:"chunkSize"`
}

func (uploader DeviceFileUploader) UploadFile(ctx context.Context, request FileUploadRequest) (UploadedFile, error) {
	var createResponse fileUploadCreateResponse
	if errorValue := uploader.DeviceClient.PostSignedJSON(uploader.DeviceClient.State.DeviceURL+"/_internkim/companion/files/uploads", map[string]any{
		"jobID":       request.JobID,
		"filename":    request.Filename,
		"sizeBytes":   request.SizeBytes,
		"contentType": request.ContentType,
		"ttlSeconds":  request.TTLSeconds,
	}, &createResponse); errorValue != nil {
		return UploadedFile{}, errorValue
	}
	chunkSize := createResponse.ChunkSize
	if chunkSize <= 0 {
		return UploadedFile{}, errors.New("device returned invalid upload chunk size")
	}
	chunks, errorValue := uploader.uploadFileChunks(ctx, createResponse.UploadID, request.Path, chunkSize)
	if errorValue != nil {
		return UploadedFile{}, errorValue
	}
	var uploadedFile UploadedFile
	if errorValue := uploader.DeviceClient.PostSignedJSON(uploader.DeviceClient.State.DeviceURL+"/_internkim/companion/files/uploads/"+url.PathEscape(createResponse.UploadID)+"/complete", map[string]any{
		"chunks": chunks,
	}, &uploadedFile); errorValue != nil {
		return UploadedFile{}, errorValue
	}
	return uploadedFile, nil
}

func (uploader DeviceFileUploader) uploadFileChunks(ctx context.Context, uploadID string, path string, chunkSize int64) (int, error) {
	file, errorValue := os.Open(path)
	if errorValue != nil {
		return 0, errors.New("selected file cannot be opened")
	}
	defer file.Close()
	buffer := make([]byte, chunkSize)
	chunkIndex := 0
	for {
		count, readError := io.ReadFull(file, buffer)
		if count > 0 {
			endpoint := uploader.DeviceClient.State.DeviceURL + "/_internkim/companion/files/uploads/" + url.PathEscape(uploadID) + "/chunks/" + strconv.Itoa(chunkIndex)
			if errorValue := uploader.DeviceClient.PutSignedBytes(ctx, endpoint, buffer[:count]); errorValue != nil {
				return chunkIndex, errorValue
			}
			chunkIndex++
		}
		if errors.Is(readError, io.EOF) || errors.Is(readError, io.ErrUnexpectedEOF) {
			return chunkIndex, nil
		}
		if readError != nil {
			return chunkIndex, errors.New("selected file cannot be read")
		}
	}
}
