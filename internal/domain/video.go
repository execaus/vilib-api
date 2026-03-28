package domain

import (
	"time"

	"github.com/google/uuid"
)

type (
	PreflightURL  string
	VideoBucket   uint
	VideoStatus   uint
	VideoAssetTag uint
)

const (
	VideoUploadURLTTL = time.Hour
	VideoStreamURLTTL = time.Hour
	DefaultVideoName  = "untitled"
)

const (
	VideoBucketOriginal VideoBucket = iota
)

const (
	VideoStatusUploading VideoStatus = iota
	VideoStatusCompressing
	VideoStatusReady
)

const (
	VideoAssetTagOriginal VideoAssetTag = iota
	VideoAssetTagCompressed
)

type Video struct {
	ID        uuid.UUID
	GroupID   uuid.UUID
	Name      string
	Author    uuid.UUID
	Status    VideoStatus
	CreatedAt time.Time
}

type VideoAsset struct {
	FileID    uuid.UUID
	VideoID   uuid.UUID
	Tag       VideoAssetTag
	CreatedAt time.Time
}
