// Copyright 2020 Northern.tech AS
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package model

import (
	"io"
	"time"

	"github.com/asaskevich/govalidator"
)

// Information provided by the user
type SoftwareImageMetaConstructor struct {
	// Image description
	Description string `json:"description,omitempty" valid:"length(1|4096),optional"`
}

// Creates new, empty SoftwareImageMetaConstructor
func NewSoftwareImageMetaConstructor() *SoftwareImageMetaConstructor {
	return &SoftwareImageMetaConstructor{}
}

// Validate checks structure according to valid tags.
func (s *SoftwareImageMetaConstructor) Validate() error {
	_, err := govalidator.ValidateStruct(s)
	return err
}

// Structure with artifact version information
type ArtifactInfo struct {
	// Mender artifact format - the only possible value is "mender"
	//Format string `json:"format" valid:"string,equal("mender"),required"`
	Format string `json:"format" valid:"required"`

	// Mender artifact format version
	//Version uint `json:"version" valid:"uint,equal(1),required"`
	Version uint `json:"version" valid:"required"`
}

// Information provided with YOCTO image
type SoftwareImageMetaArtifactConstructor struct {
	// artifact_name from artifact file
	Name string `json:"name" bson:"name" valid:"length(1|4096),required"`

	// Compatible device types for the application
	DeviceTypesCompatible []string `json:"device_types_compatible" bson:"device_types_compatible" valid:"length(1|4096),required"`

	// Artifact version info
	Info *ArtifactInfo `json:"info"`

	// Flag that indicates if artifact is signed or not
	Signed bool `json:"signed" bson:"signed"`

	// List of updates
	Updates []Update `json:"updates" valid:"-"`
}

func NewSoftwareImageMetaArtifactConstructor() *SoftwareImageMetaArtifactConstructor {
	return &SoftwareImageMetaArtifactConstructor{}
}

// Validate checks structure according to valid tags.
func (s *SoftwareImageMetaArtifactConstructor) Validate() error {
	_, err := govalidator.ValidateStruct(s)
	return err
}

// SoftwareImage YOCTO image with user application
type SoftwareImage struct {
	// User provided field set
	SoftwareImageMetaConstructor `bson:"meta"`

	// Field set provided with yocto image
	SoftwareImageMetaArtifactConstructor `bson:"meta_artifact"`

	// Image ID
	Id string `json:"id" bson:"_id" valid:"uuidv4,required"`

	// Artifact total size
	Size int64 `json:"size" bson:"size" valid:"-"`

	// Last modification time, including image upload time
	Modified *time.Time `json:"modified" valid:"-"`
}

// NewSoftwareImage creates new software image object.
func NewSoftwareImage(
	id string,
	metaConstructor *SoftwareImageMetaConstructor,
	metaArtifactConstructor *SoftwareImageMetaArtifactConstructor,
	artifactSize int64) *SoftwareImage {

	now := time.Now()

	return &SoftwareImage{
		SoftwareImageMetaConstructor:         *metaConstructor,
		SoftwareImageMetaArtifactConstructor: *metaArtifactConstructor,
		Modified:                             &now,
		Id:                                   id,
		Size:                                 artifactSize,
	}
}

// SetModified set last modification time for the image.
func (s *SoftwareImage) SetModified(time time.Time) {
	s.Modified = &time
}

// Validate checks structure according to valid tags.
func (s *SoftwareImage) Validate() error {
	_, err := govalidator.ValidateStruct(s)
	return err
}

// MultipartUploadMsg is a structure with fields extracted from the multipart/form-data form
// send in the artifact upload request
type MultipartUploadMsg struct {
	// user metadata constructor
	MetaConstructor *SoftwareImageMetaConstructor
	// ArtifactID contains the artifact ID
	ArtifactID string
	// size of the artifact file
	ArtifactSize int64
	// reader pointing to the beginning of the artifact data
	ArtifactReader io.ReadCloser
}

// MultipartGenerateImageMsg is a structure with fields extracted from the multipart/form-data
// form sent in the artifact generation request
type MultipartGenerateImageMsg struct {
	Name                  string    `json:"name"`
	Description           string    `json:"description"`
	Size                  int64     `json:"size"`
	DeviceTypesCompatible []string  `json:"device_types_compatible"`
	Type                  string    `json:"type"`
	Args                  string    `json:"args"`
	ArtifactID            string    `json:"artifact_id"`
	GetArtifactURI        string    `json:"get_artifact_uri"`
	DeleteArtifactURI     string    `json:"delete_artifact_uri"`
	TenantID              string    `json:"tenant_id"`
	Token                 string    `json:"token"`
	FileReader            io.Reader `json:"-"`
}
