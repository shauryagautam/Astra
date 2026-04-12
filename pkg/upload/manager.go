package upload

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// File represents an uploaded file
type File struct {
	ID          string            `json:"id"`
	Filename    string            `json:"filename"`
	OriginalName string           `json:"original_name"`
	Size        int64             `json:"size"`
	MimeType    string            `json:"mime_type"`
	Extension   string            `json:"extension"`
	Path        string            `json:"path"`
	URL         string            `json:"url"`
	Hash        string            `json:"hash"`
	UploadedBy  string            `json:"uploaded_by"`
	TenantID    string            `json:"tenant_id"`
	IsPublic    bool              `json:"is_public"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// UploadConfig represents upload configuration
type UploadConfig struct {
	MaxFileSize      int64             `json:"max_file_size"`
	AllowedTypes     []string          `json:"allowed_types"`
	AllowedExts      []string          `json:"allowed_extensions"`
	StoragePath      string            `json:"storage_path"`
	PublicURL        string            `json:"public_url"`
	EnableProcessing bool              `json:"enable_processing"`
	ImageConfig      *ImageConfig      `json:"image_config"`
	VideoConfig      *VideoConfig      `json:"video_config"`
	DocumentConfig   *DocumentConfig   `json:"document_config"`
}

// ImageConfig represents image processing configuration
type ImageConfig struct {
	EnableResize     bool     `json:"enable_resize"`
	EnableThumbnail  bool     `json:"enable_thumbnail"`
	MaxWidth         int      `json:"max_width"`
	MaxHeight        int      `json:"max_height"`
	ThumbnailSize    int      `json:"thumbnail_size"`
	Quality          int      `json:"quality"`
	Formats          []string `json:"formats"`
}

// VideoConfig represents video processing configuration
type VideoConfig struct {
	EnableTranscoding bool     `json:"enable_transcoding"`
	MaxResolution     string   `json:"max_resolution"`
	Formats           []string `json:"formats"`
	ThumbnailTime     float64  `json:"thumbnail_time"`
}

// DocumentConfig represents document processing configuration
type DocumentConfig struct {
	EnableOCR        bool     `json:"enable_ocr"`
	EnablePreview    bool     `json:"enable_preview"`
	SupportedFormats []string `json:"supported_formats"`
}

// Storage interface for file storage
type Storage interface {
	// File operations
	Store(ctx context.Context, file *File, content io.Reader) error
	Get(ctx context.Context, id string) (*File, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, userID, tenantID string) ([]*File, error)
	
	// Content operations
	GetContent(ctx context.Context, path string) (io.ReadCloser, error)
	GetURL(ctx context.Context, path string) (string, error)
	
	// Metadata operations
	UpdateMetadata(ctx context.Context, id string, metadata map[string]string) error
}

// Processor interface for media processing
type Processor interface {
	ProcessImage(ctx context.Context, file *File, config *ImageConfig) error
	ProcessVideo(ctx context.Context, file *File, config *VideoConfig) error
	ProcessDocument(ctx context.Context, file *File, config *DocumentConfig) error
	GenerateThumbnail(ctx context.Context, file *File) error
}

// Logger interface for logging
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
}

// Manager manages file uploads
type Manager struct {
	storage   Storage
	processor Processor
	logger    Logger
	config    UploadConfig
}

// NewManager creates a new upload manager
func NewManager(storage Storage, processor Processor, logger Logger, config UploadConfig) *Manager {
	return &Manager{
		storage:   storage,
		processor: processor,
		logger:    logger,
		config:    config,
	}
}

// UploadFile handles file upload
func (m *Manager) UploadFile(ctx context.Context, header *multipart.FileHeader, uploadedBy, tenantID string, isPublic bool) (*File, error) {
	// Validate file
	if err := m.validateFile(header); err != nil {
		return nil, fmt.Errorf("file validation failed: %w", err)
	}

	// Open file
	file, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Calculate file hash
	hash, err := m.calculateHash(file)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate file hash: %w", err)
	}

	// Reset file reader
	file.Seek(0, 0)

	// Generate file path and URL
	filePath := m.generateFilePath(header.Filename, hash)
	fileURL := m.generateURL(filePath)

	// Create file record
	uploadFile := &File{
		ID:           m.generateID(),
		Filename:     filepath.Base(filePath),
		OriginalName: header.Filename,
		Size:         header.Size,
		MimeType:     header.Header.Get("Content-Type"),
		Extension:    strings.ToLower(filepath.Ext(header.Filename)),
		Path:         filePath,
		URL:          fileURL,
		Hash:         hash,
		UploadedBy:   uploadedBy,
		TenantID:     tenantID,
		IsPublic:     isPublic,
		Metadata:     make(map[string]string),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Store file
	if err := m.storage.Store(ctx, uploadFile, file); err != nil {
		return nil, fmt.Errorf("failed to store file: %w", err)
	}

	// Process file if enabled
	if m.config.EnableProcessing {
		if err := m.processFile(ctx, uploadFile); err != nil {
			m.logger.Error("File processing failed", "error", err, "file_id", uploadFile.ID)
			// Don't fail the upload, just log the error
		}
	}

	m.logger.Info("File uploaded", 
		"file_id", uploadFile.ID,
		"filename", uploadFile.OriginalName,
		"size", uploadFile.Size,
		"user_id", uploadedBy,
		"tenant_id", tenantID,
	)

	return uploadFile, nil
}

// GetFile retrieves a file by ID
func (m *Manager) GetFile(ctx context.Context, id string) (*File, error) {
	file, err := m.storage.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	return file, nil
}

// DeleteFile deletes a file
func (m *Manager) DeleteFile(ctx context.Context, id string) error {
	if err := m.storage.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	m.logger.Info("File deleted", "file_id", id)
	return nil
}

// ListFiles returns files for a user or tenant
func (m *Manager) ListFiles(ctx context.Context, userID, tenantID string) ([]*File, error) {
	files, err := m.storage.List(ctx, userID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}
	return files, nil
}

// GetFileContent returns file content
func (m *Manager) GetFileContent(ctx context.Context, id string) (io.ReadCloser, error) {
	file, err := m.storage.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	content, err := m.storage.GetContent(ctx, file.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to get file content: %w", err)
	}

	return content, nil
}

// UpdateFileMetadata updates file metadata
func (m *Manager) UpdateFileMetadata(ctx context.Context, id string, metadata map[string]string) error {
	if err := m.storage.UpdateMetadata(ctx, id, metadata); err != nil {
		return fmt.Errorf("failed to update file metadata: %w", err)
	}

	m.logger.Info("File metadata updated", "file_id", id)
	return nil
}

// validateFile validates uploaded file
func (m *Manager) validateFile(header *multipart.FileHeader) error {
	// Check file size
	if m.config.MaxFileSize > 0 && header.Size > m.config.MaxFileSize {
		return fmt.Errorf("file size %d exceeds maximum allowed size %d", header.Size, m.config.MaxFileSize)
	}

	// Check MIME type
	if len(m.config.AllowedTypes) > 0 {
		mimeType := header.Header.Get("Content-Type")
		allowed := false
		for _, allowedType := range m.config.AllowedTypes {
			if strings.HasPrefix(mimeType, allowedType) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("MIME type %s is not allowed", mimeType)
		}
	}

	// Check file extension
	if len(m.config.AllowedExts) > 0 {
		ext := strings.ToLower(filepath.Ext(header.Filename))
		allowed := false
		for _, allowedExt := range m.config.AllowedExts {
			if ext == allowedExt {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("file extension %s is not allowed", ext)
		}
	}

	return nil
}

// processFile processes uploaded file
func (m *Manager) processFile(ctx context.Context, file *File) error {
	mimeType := file.MimeType

	// Process images
	if strings.HasPrefix(mimeType, "image/") && m.config.ImageConfig != nil {
		if err := m.processor.ProcessImage(ctx, file, m.config.ImageConfig); err != nil {
			return fmt.Errorf("image processing failed: %w", err)
		}

		// Generate thumbnail
		if m.config.ImageConfig.EnableThumbnail {
			if err := m.processor.GenerateThumbnail(ctx, file); err != nil {
				m.logger.Error("Thumbnail generation failed", "error", err, "file_id", file.ID)
			}
		}
	}

	// Process videos
	if strings.HasPrefix(mimeType, "video/") && m.config.VideoConfig != nil {
		if err := m.processor.ProcessVideo(ctx, file, m.config.VideoConfig); err != nil {
			return fmt.Errorf("video processing failed: %w", err)
		}

		// Generate thumbnail
		if err := m.processor.GenerateThumbnail(ctx, file); err != nil {
			m.logger.Error("Video thumbnail generation failed", "error", err, "file_id", file.ID)
		}
	}

	// Process documents
	if m.isDocument(mimeType) && m.config.DocumentConfig != nil {
		if err := m.processor.ProcessDocument(ctx, file, m.config.DocumentConfig); err != nil {
			return fmt.Errorf("document processing failed: %w", err)
		}
	}

	return nil
}

// isDocument checks if MIME type is a document
func (m *Manager) isDocument(mimeType string) bool {
	documentTypes := []string{
		"application/pdf",
		"application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.ms-powerpoint",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"text/plain",
		"text/csv",
	}

	for _, docType := range documentTypes {
		if mimeType == docType {
			return true
		}
	}

	return false
}

// calculateHash calculates MD5 hash of file content
func (m *Manager) calculateHash(file io.Reader) (string, error) {
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// generateFilePath generates storage path for file
func (m *Manager) generateFilePath(filename, hash string) string {
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)
	
	// Create directory structure by date
	now := time.Now()
	dir := fmt.Sprintf("%s/%d/%02d/%02d", 
		m.config.StoragePath, 
		now.Year(), 
		now.Month(), 
		now.Day(),
	)
	
	// Generate unique filename
	safeName := m.sanitizeFilename(name)
	filePath := fmt.Sprintf("%s/%s_%s%s", dir, safeName, hash[:8], ext)
	
	return filePath
}

// generateURL generates public URL for file
func (m *Manager) generateURL(filePath string) string {
	if m.config.PublicURL == "" {
		return ""
	}
	
	// Remove storage path from file path
	relativePath := strings.TrimPrefix(filePath, m.config.StoragePath)
	relativePath = strings.TrimPrefix(relativePath, "/")
	
	return fmt.Sprintf("%s/%s", strings.TrimSuffix(m.config.PublicURL, "/"), relativePath)
}

// sanitizeFilename sanitizes filename for storage
func (m *Manager) sanitizeFilename(filename string) string {
	// Replace problematic characters
	replacements := map[string]string{
		" ": "_",
		"/": "_",
		"\\": "_",
		":": "_",
		"*": "_",
		"?": "_",
		"\"": "_",
		"<": "_",
		">": "_",
		"|": "_",
	}

	safe := filename
	for old, new := range replacements {
		safe = strings.ReplaceAll(safe, old, new)
	}

	// Remove consecutive underscores
	safe = strings.ReplaceAll(safe, "__", "_")
	
	// Trim underscores from start and end
	safe = strings.Trim(safe, "_")
	
	// Limit length
	if len(safe) > 50 {
		safe = safe[:50]
	}

	return safe
}

// generateID generates unique file ID
func (m *Manager) generateID() string {
	return fmt.Sprintf("file_%d", time.Now().UnixNano())
}

// FileProcessor implements media processing
type FileProcessor struct {
	logger Logger
}

// NewFileProcessor creates a new file processor
func NewFileProcessor(logger Logger) *FileProcessor {
	return &FileProcessor{
		logger: logger,
	}
}

// ProcessImage processes image files
func (fp *FileProcessor) ProcessImage(ctx context.Context, file *File, config *ImageConfig) error {
	// This is a placeholder implementation
	// In practice, you would use image processing libraries like:
	// - github.com/disintegration/imaging
	// - github.com/nfnt/resize
	// - github.com/disintegration/gift
	
	fp.logger.Info("Processing image", "file_id", file.ID, "filename", file.Filename)
	
	// Add processing metadata
	if file.Metadata == nil {
		file.Metadata = make(map[string]string)
	}
	file.Metadata["processed"] = "true"
	file.Metadata["processed_at"] = time.Now().Format(time.RFC3339)
	
	return nil
}

// ProcessVideo processes video files
func (fp *FileProcessor) ProcessVideo(ctx context.Context, file *File, config *VideoConfig) error {
	// This is a placeholder implementation
	// In practice, you would use video processing libraries like:
	// - github.com/giorgisio/goav
	// - ffmpeg command-line tool
	
	fp.logger.Info("Processing video", "file_id", file.ID, "filename", file.Filename)
	
	// Add processing metadata
	if file.Metadata == nil {
		file.Metadata = make(map[string]string)
	}
	file.Metadata["processed"] = "true"
	file.Metadata["processed_at"] = time.Now().Format(time.RFC3339)
	
	return nil
}

// ProcessDocument processes document files
func (fp *FileProcessor) ProcessDocument(ctx context.Context, file *File, config *DocumentConfig) error {
	// This is a placeholder implementation
	// In practice, you would use document processing libraries like:
	// - github.com/unidoc/unioffice for Office documents
	// - github.com/ledongthuc/pdf for PDF
	
	fp.logger.Info("Processing document", "file_id", file.ID, "filename", file.Filename)
	
	// Add processing metadata
	if file.Metadata == nil {
		file.Metadata = make(map[string]string)
	}
	file.Metadata["processed"] = "true"
	file.Metadata["processed_at"] = time.Now().Format(time.RFC3339)
	
	return nil
}

// GenerateThumbnail generates thumbnail for media files
func (fp *FileProcessor) GenerateThumbnail(ctx context.Context, file *File) error {
	// This is a placeholder implementation
	// In practice, you would generate actual thumbnail files
	
	fp.logger.Info("Generating thumbnail", "file_id", file.ID, "filename", file.Filename)
	
	// Add thumbnail metadata
	if file.Metadata == nil {
		file.Metadata = make(map[string]string)
	}
	file.Metadata["thumbnail"] = "true"
	file.Metadata["thumbnail_at"] = time.Now().Format(time.RFC3339)
	
	return nil
}

// FileSystemStorage implements file system storage
type FileSystemStorage struct {
	basePath  string
	publicURL string
	logger    Logger
}

// NewFileSystemStorage creates new file system storage
func NewFileSystemStorage(basePath, publicURL string, logger Logger) *FileSystemStorage {
	return &FileSystemStorage{
		basePath:  basePath,
		publicURL: publicURL,
		logger:    logger,
	}
}

// Store stores file in file system
func (fs *FileSystemStorage) Store(ctx context.Context, file *File, content io.Reader) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(file.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file
	f, err := os.Create(file.Path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	// Copy content
	if _, err := io.Copy(f, content); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Get retrieves file metadata
func (fs *FileSystemStorage) Get(ctx context.Context, id string) (*File, error) {
	// This is a placeholder - in practice, you would store metadata in a database
	return nil, fmt.Errorf("not implemented")
}

// Delete deletes file
func (fs *FileSystemStorage) Delete(ctx context.Context, id string) error {
	// This is a placeholder - in practice, you would get path from database and delete
	return fmt.Errorf("not implemented")
}

// List files
func (fs *FileSystemStorage) List(ctx context.Context, userID, tenantID string) ([]*File, error) {
	// This is a placeholder - in practice, you would query database
	return nil, fmt.Errorf("not implemented")
}

// GetContent returns file content
func (fs *FileSystemStorage) GetContent(ctx context.Context, path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	return f, nil
}

// GetURL returns file URL
func (fs *FileSystemStorage) GetURL(ctx context.Context, path string) (string, error) {
	if fs.publicURL == "" {
		return "", fmt.Errorf("public URL not configured")
	}
	
	relativePath := strings.TrimPrefix(path, fs.basePath)
	relativePath = strings.TrimPrefix(relativePath, "/")
	
	return fmt.Sprintf("%s/%s", strings.TrimSuffix(fs.publicURL, "/"), relativePath), nil
}

// UpdateMetadata updates file metadata
func (fs *FileSystemStorage) UpdateMetadata(ctx context.Context, id string, metadata map[string]string) error {
	// This is a placeholder - in practice, you would update database
	return fmt.Errorf("not implemented")
}
