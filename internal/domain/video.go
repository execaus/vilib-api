package domain

import (
	"time"
	"vilib-api/internal/gen/schema"

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
	VideoBucketOriginal   VideoBucket = iota
	VideoBucketCompressed VideoBucket = iota
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

func (v *Video) FromDB(db *schema.UserGroupVideo) {
	v.ID = db.ID
	v.GroupID = db.UserGroupID
	v.Name = db.Name
	v.Author = db.Author
	v.Status = VideoStatus(db.Status)
	v.CreatedAt = db.CreatedAt
}

func (va *VideoAsset) FromDB(db *schema.VideoAsset) {
	va.FileID = db.FileID
	va.VideoID = db.VideoID
	va.Tag = VideoAssetTag(db.Tag)
	va.CreatedAt = db.CreatedAt
}
