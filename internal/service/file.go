package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pb "github.com/J-Y-Zhang/light-cloud-disk/api/file/v1"
	"github.com/J-Y-Zhang/light-cloud-disk/internal/biz"
)

type FileService struct {
	pb.UnimplementedFileServiceServer

	uc  *biz.FileUsecase
	log *log.Helper
}

func NewFileService(uc *biz.FileUsecase, logger log.Logger) *FileService {
	return &FileService{
		uc:  uc,
		log: log.NewHelper(logger),
	}
}

func (s *FileService) CheckUpload(ctx context.Context, req *pb.CheckUploadRequest) (*pb.CheckUploadReply, error) {
	canFastUpload, uploadedChunks, err := s.uc.CheckUpload(ctx, req.FileMd5, req.FileSize, req.TotalChunks)
	if err != nil {
		return nil, err
	}
	return &pb.CheckUploadReply{
		CanFastUpload:  canFastUpload,
		UploadedChunks: uploadedChunks,
	}, nil
}

func (s *FileService) UploadChunk(ctx context.Context, req *pb.UploadChunkRequest) (*pb.UploadChunkReply, error) {
	// TODO: Save chunk to temp directory
	err := s.uc.SaveChunk(ctx, req.FileMd5, req.ChunkIndex, int64(req.ChunkSize))
	if err != nil {
		return nil, err
	}
	return &pb.UploadChunkReply{
		Success:    true,
		ChunkIndex: req.ChunkIndex,
	}, nil
}

func (s *FileService) MergeChunks(ctx context.Context, req *pb.MergeChunksRequest) (*pb.MergeChunksReply, error) {
	file, err := s.uc.MergeChunks(ctx, req.UserId, req.ParentId, req.FileName, req.FileMd5, req.FileSize)
	if err != nil {
		return nil, err
	}
	return &pb.MergeChunksReply{
		Success: true,
		File:    s.fileToProto(file),
	}, nil
}

func (s *FileService) ListFiles(ctx context.Context, req *pb.ListFilesRequest) (*pb.ListFilesReply, error) {
	files, total, err := s.uc.ListFiles(ctx, req.UserId, req.ParentId, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}

	pbFiles := make([]*pb.FileInfo, len(files))
	for i, f := range files {
		pbFiles[i] = s.fileToProto(f)
	}

	return &pb.ListFilesReply{
		Files: pbFiles,
		Total: total,
	}, nil
}

func (s *FileService) GetDownloadURL(ctx context.Context, req *pb.GetDownloadURLRequest) (*pb.GetDownloadURLReply, error) {
	// TODO: Generate download URL from OSS or local storage
	return &pb.GetDownloadURLReply{
		DownloadUrl: "",
		FileName:    "",
	}, nil
}

func (s *FileService) DeleteFile(ctx context.Context, req *pb.DeleteFileRequest) (*pb.DeleteFileReply, error) {
	err := s.uc.DeleteFiles(ctx, req.UserId, req.FileIds)
	if err != nil {
		return nil, err
	}
	return &pb.DeleteFileReply{Success: true}, nil
}

func (s *FileService) RenameFile(ctx context.Context, req *pb.RenameFileRequest) (*pb.RenameFileReply, error) {
	err := s.uc.RenameFile(ctx, req.UserId, req.FileId, req.NewName)
	if err != nil {
		return nil, err
	}
	return &pb.RenameFileReply{Success: true}, nil
}

func (s *FileService) CreateFolder(ctx context.Context, req *pb.CreateFolderRequest) (*pb.CreateFolderReply, error) {
	folder, err := s.uc.CreateFolder(ctx, req.UserId, req.ParentId, req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.CreateFolderReply{
		Success: true,
		Folder:  s.fileToProto(folder),
	}, nil
}

func (s *FileService) MoveFile(ctx context.Context, req *pb.MoveFileRequest) (*pb.MoveFileReply, error) {
	err := s.uc.MoveFiles(ctx, req.UserId, req.FileIds, req.TargetFolderId)
	if err != nil {
		return nil, err
	}
	return &pb.MoveFileReply{Success: true}, nil
}

func (s *FileService) ListTrash(ctx context.Context, req *pb.ListTrashRequest) (*pb.ListTrashReply, error) {
	files, total, err := s.uc.ListTrash(ctx, req.UserId, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}

	pbFiles := make([]*pb.TrashFileInfo, len(files))
	for i, f := range files {
		pbFiles[i] = &pb.TrashFileInfo{
			Id:        f.ID,
			Name:      f.Name,
			Size:      f.Size,
			IsFolder:  f.IsFolder,
			DeletedAt: f.DeletedAt.Unix(),
		}
	}

	return &pb.ListTrashReply{
		Files: pbFiles,
		Total: total,
	}, nil
}

func (s *FileService) RestoreFile(ctx context.Context, req *pb.RestoreFileRequest) (*pb.RestoreFileReply, error) {
	err := s.uc.RestoreFiles(ctx, req.UserId, req.FileIds)
	if err != nil {
		return nil, err
	}
	return &pb.RestoreFileReply{Success: true}, nil
}

func (s *FileService) PermanentDelete(ctx context.Context, req *pb.PermanentDeleteRequest) (*pb.PermanentDeleteReply, error) {
	err := s.uc.PermanentDelete(ctx, req.UserId, req.FileIds)
	if err != nil {
		return nil, err
	}
	return &pb.PermanentDeleteReply{Success: true}, nil
}

func (s *FileService) CreateShare(ctx context.Context, req *pb.CreateShareRequest) (*pb.CreateShareReply, error) {
	share, err := s.uc.CreateShare(ctx, req.UserId, req.FileId, req.ExpireDays, req.Password)
	if err != nil {
		return nil, err
	}

	var expireAt int64
	if share.ExpireAt != nil {
		expireAt = share.ExpireAt.Unix()
	}

	return &pb.CreateShareReply{
		ShareId:  share.ID,
		ShareUrl: "/s/" + share.ID,
		Password: share.Password,
		ExpireAt: expireAt,
	}, nil
}

func (s *FileService) GetShare(ctx context.Context, req *pb.GetShareRequest) (*pb.GetShareReply, error) {
	file, err := s.uc.GetShare(ctx, req.ShareId, req.Password)
	if err != nil {
		return nil, err
	}
	return &pb.GetShareReply{
		File: s.fileToProto(file),
	}, nil
}

func (s *FileService) SearchFiles(ctx context.Context, req *pb.SearchFilesRequest) (*pb.SearchFilesReply, error) {
	files, total, err := s.uc.SearchFiles(ctx, req.UserId, req.Keyword, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}

	pbFiles := make([]*pb.FileInfo, len(files))
	for i, f := range files {
		pbFiles[i] = s.fileToProto(f)
	}

	return &pb.SearchFilesReply{
		Files: pbFiles,
		Total: total,
	}, nil
}

func (s *FileService) fileToProto(f *biz.File) *pb.FileInfo {
	return &pb.FileInfo{
		Id:        f.ID,
		Name:      f.Name,
		FileMd5:   f.FileMD5,
		Size:      f.Size,
		IsFolder:  f.IsFolder,
		ParentId:  f.ParentID,
		Path:      f.Path,
		CreatedAt: f.CreatedAt.Unix(),
		UpdatedAt: f.UpdatedAt.Unix(),
	}
}
