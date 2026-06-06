package handler

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zhanuzak/microservices/media-service/internal/storage"
	pb "github.com/zhanuzak/microservices/media-service/proto/media"
)

type GRPCMediaHandler struct {
	pb.UnimplementedMediaServiceServer
	storage *storage.MinioStorage
}

func NewGRPCMediaHandler(s *storage.MinioStorage) *GRPCMediaHandler {
	return &GRPCMediaHandler{storage: s}
}

func (h *GRPCMediaHandler) UploadFile(ctx context.Context, req *pb.UploadFileRequest) (*pb.UploadFileResponse, error) {
	if len(req.Data) == 0 {
		return nil, status.Error(codes.InvalidArgument, "file data is required")
	}
	if req.Filename == "" {
		return nil, status.Error(codes.InvalidArgument, "filename is required")
	}

	contentType := req.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if !allowedTypes[contentType] {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("unsupported file type: %s", contentType))
	}

	if len(req.Data) > maxFileSize {
		return nil, status.Error(codes.InvalidArgument, "file too large (max 10MB)")
	}

	folder := req.Folder
	if folder == "" {
		folder = "uploads"
	}

	ext := ""
	if idx := strings.LastIndex(req.Filename, "."); idx != -1 {
		ext = strings.ToLower(req.Filename[idx:])
	}
	key := fmt.Sprintf("%s/%s%s", folder, uuid.New().String(), ext)

	reader := bytes.NewReader(req.Data)
	url, err := h.storage.Upload(ctx, key, reader, int64(len(req.Data)), contentType)
	if err != nil {
		log.Printf("ERROR: failed to upload file: %v", err)
		return nil, status.Error(codes.Internal, "failed to upload file")
	}

	return &pb.UploadFileResponse{
		Key: key,
		Url: url,
	}, nil
}

func (h *GRPCMediaHandler) DeleteFile(ctx context.Context, req *pb.DeleteFileRequest) (*pb.DeleteFileResponse, error) {
	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "key is required")
	}

	if err := h.storage.Delete(ctx, req.Key); err != nil {
		log.Printf("ERROR: failed to delete file: %v", err)
		return nil, status.Error(codes.Internal, "failed to delete file")
	}

	return &pb.DeleteFileResponse{Success: true}, nil
}

func (h *GRPCMediaHandler) GetFileURL(ctx context.Context, req *pb.GetFileURLRequest) (*pb.GetFileURLResponse, error) {
	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "key is required")
	}

	url := h.storage.FileURL(req.Key)
	return &pb.GetFileURLResponse{Url: url}, nil
}
