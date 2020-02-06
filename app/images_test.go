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

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/mendersoftware/deployments/client/workflows"
	workflows_mocks "github.com/mendersoftware/deployments/client/workflows/mocks"
	"github.com/mendersoftware/deployments/model"
	fs_mocks "github.com/mendersoftware/deployments/s3/mocks"
	"github.com/mendersoftware/deployments/store/mocks"
	h "github.com/mendersoftware/deployments/utils/testing"
	"github.com/mendersoftware/go-lib-micro/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGenerateArtifactError(t *testing.T) {
	db := mocks.DataStore{}
	fs := &fs_mocks.FileStorage{}
	d := NewDeployments(&db, fs, ArtifactContentType)

	testCases := []struct {
		multipartGenerateArtifact *model.MultipartGenerateArtifactMsg
		expectedError          error
	}{
		{
			multipartGenerateArtifact: nil,
			expectedError:          ErrModelMultipartUploadMsgMalformed,
		},
		{
			multipartGenerateArtifact: &model.MultipartGenerateArtifactMsg{
				Size: MaxImageSize + 1,
			},
			expectedError: ErrModelArtifactFileTooLarge,
		},
	}

	ctx := context.Background()
	for i := range testCases {
		tc := testCases[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			artifactID, err := d.GenerateArtifact(ctx, tc.multipartGenerateArtifact)

			assert.Equal(t, artifactID, "")
			assert.Error(t, err)
			assert.EqualError(t, err, tc.expectedError.Error())
		})
	}
}

func TestGenerateArtifactArtifactIsNotUnique(t *testing.T) {
	db := mocks.DataStore{}
	fs := &fs_mocks.FileStorage{}
	d := NewDeployments(&db, fs, ArtifactContentType)

	db.On("IsArtifactUnique",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string"),
	).Return(false, nil)

	multipartGenerateArtifact := &model.MultipartGenerateArtifactMsg{
		Name:                  "name",
		Description:           "description",
		DeviceTypesCompatible: []string{"Beagle Bone"},
		Type:                  "single_file",
		Args:                  "",
		Size:                  10,
		FileReader:            nil,
	}

	ctx := context.Background()
	artifactID, err := d.GenerateArtifact(ctx, multipartGenerateArtifact)

	assert.Equal(t, artifactID, "")
	assert.Error(t, err)
	assert.EqualError(t, err, ErrModelArtifactNotUnique.Error())

	db.AssertExpectations(t)
}

func TestGenerateArtifactErrorWhileCheckingIfArtifactIsNotUnique(t *testing.T) {
	db := mocks.DataStore{}
	fs := &fs_mocks.FileStorage{}
	d := NewDeployments(&db, fs, ArtifactContentType)

	db.On("IsArtifactUnique",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string"),
	).Return(false, errors.New("error"))

	multipartGenerateArtifact := &model.MultipartGenerateArtifactMsg{
		Name:                  "name",
		Description:           "description",
		DeviceTypesCompatible: []string{"Beagle Bone"},
		Type:                  "single_file",
		Args:                  "",
		Size:                  10,
		FileReader:            nil,
	}

	ctx := context.Background()
	artifactID, err := d.GenerateArtifact(ctx, multipartGenerateArtifact)

	assert.Equal(t, artifactID, "")
	assert.Error(t, err)
	assert.EqualError(t, err, "Fail to check if artifact is unique: error")

	db.AssertExpectations(t)
}

func TestGenerateArtifactErrorWhileUploading(t *testing.T) {
	db := mocks.DataStore{}
	fs := &fs_mocks.FileStorage{}
	d := NewDeployments(&db, fs, ArtifactContentType)

	fs.On("UploadArtifact",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("int64"),
		mock.AnythingOfType("*io.LimitedReader"),
		mock.AnythingOfType("string"),
	).Return(errors.New("error while uploading"))

	db.On("IsArtifactUnique",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string"),
	).Return(true, nil)

	multipartGenerateArtifact := &model.MultipartGenerateArtifactMsg{
		Name:                  "name",
		Description:           "description",
		DeviceTypesCompatible: []string{"Beagle Bone"},
		Type:                  "single_file",
		Args:                  "",
		Size:                  10,
		FileReader:            bytes.NewReader([]byte("123456790")),
	}

	ctx := context.Background()
	artifactID, err := d.GenerateArtifact(ctx, multipartGenerateArtifact)

	assert.Equal(t, artifactID, "")
	assert.Error(t, err)
	assert.EqualError(t, err, "error while uploading")

	db.AssertExpectations(t)
	fs.AssertExpectations(t)
}

func TestGenerateArtifactErrorS3GetRequest(t *testing.T) {
	db := mocks.DataStore{}
	fs := &fs_mocks.FileStorage{}
	d := NewDeployments(&db, fs, ArtifactContentType)

	fs.On("UploadArtifact",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("int64"),
		mock.AnythingOfType("*io.LimitedReader"),
		mock.AnythingOfType("string"),
	).Return(nil)

	db.On("IsArtifactUnique",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string"),
	).Return(true, nil)

	fs.On("GetRequest",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("time.Duration"),
		mock.AnythingOfType("string"),
	).Return(nil, errors.New("error get request"))

	multipartGenerateArtifact := &model.MultipartGenerateArtifactMsg{
		Name:                  "name",
		Description:           "description",
		DeviceTypesCompatible: []string{"Beagle Bone"},
		Type:                  "single_file",
		Args:                  "",
		Size:                  10,
		FileReader:            bytes.NewReader([]byte("123456790")),
	}

	ctx := context.Background()
	artifactID, err := d.GenerateArtifact(ctx, multipartGenerateArtifact)

	assert.Equal(t, artifactID, "")
	assert.Error(t, err)
	assert.EqualError(t, err, "error get request")

	db.AssertExpectations(t)
	fs.AssertExpectations(t)
}

func TestGenerateArtifactErrorS3DeleteRequest(t *testing.T) {
	db := mocks.DataStore{}
	fs := &fs_mocks.FileStorage{}
	d := NewDeployments(&db, fs, ArtifactContentType)

	fs.On("UploadArtifact",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("int64"),
		mock.AnythingOfType("*io.LimitedReader"),
		mock.AnythingOfType("string"),
	).Return(nil)

	db.On("IsArtifactUnique",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string"),
	).Return(true, nil)

	fs.On("GetRequest",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("time.Duration"),
		mock.AnythingOfType("string"),
	).Return(&model.Link{
		Uri: "GET",
	}, nil)

	fs.On("DeleteRequest",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("time.Duration"),
	).Return(nil, errors.New("error delete request"))

	multipartGenerateArtifact := &model.MultipartGenerateArtifactMsg{
		Name:                  "name",
		Description:           "description",
		DeviceTypesCompatible: []string{"Beagle Bone"},
		Type:                  "single_file",
		Args:                  "",
		Size:                  10,
		FileReader:            bytes.NewReader([]byte("123456790")),
	}

	ctx := context.Background()
	artifactID, err := d.GenerateArtifact(ctx, multipartGenerateArtifact)

	assert.Equal(t, artifactID, "")
	assert.Error(t, err)
	assert.EqualError(t, err, "error delete request")

	db.AssertExpectations(t)
	fs.AssertExpectations(t)
}

func TestGenerateArtifactErrorWhileStartingWorkflow(t *testing.T) {
	db := mocks.DataStore{}
	fs := &fs_mocks.FileStorage{}
	d := NewDeployments(&db, fs, ArtifactContentType)

	mockHTTPClient := &workflows_mocks.HTTPClientMock{}
	mockHTTPClient.On("Do",
		mock.AnythingOfType("*http.Request"),
	).Return(&http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       ioutil.NopCloser(strings.NewReader("")),
	}, nil)

	fs.On("GetRequest",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("time.Duration"),
		mock.AnythingOfType("string"),
	).Return(&model.Link{
		Uri: "GET",
	}, nil)

	fs.On("DeleteRequest",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("time.Duration"),
		mock.AnythingOfType("string"),
	).Return(&model.Link{
		Uri: "DELETE",
	}, nil)

	workflowsClient := workflows.NewClient()
	workflowsClient.SetHTTPClient(mockHTTPClient)
	d.SetWorkflowsClient(workflowsClient)

	fs.On("UploadArtifact",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("int64"),
		mock.AnythingOfType("*io.LimitedReader"),
		mock.AnythingOfType("string"),
	).Return(nil)

	fs.On("Delete",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
	).Return(nil)

	db.On("IsArtifactUnique",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string"),
	).Return(true, nil)

	multipartGenerateArtifact := &model.MultipartGenerateArtifactMsg{
		Name:                  "name",
		Description:           "description",
		DeviceTypesCompatible: []string{"Beagle Bone"},
		Type:                  "single_file",
		Args:                  "",
		Size:                  10,
		FileReader:            bytes.NewReader([]byte("123456790")),
	}

	ctx := context.Background()
	artifactID, err := d.GenerateArtifact(ctx, multipartGenerateArtifact)

	assert.Equal(t, artifactID, "")
	assert.Error(t, err)
	assert.EqualError(t, err, "failed to start workflow: generate_artifact")

	db.AssertExpectations(t)
	fs.AssertExpectations(t)
	mockHTTPClient.AssertExpectations(t)
}

func TestGenerateArtifactErrorWhileStartingWorkflowAndFailsWhenCleaningUp(t *testing.T) {
	db := mocks.DataStore{}
	fs := &fs_mocks.FileStorage{}
	d := NewDeployments(&db, fs, ArtifactContentType)

	mockHTTPClient := &workflows_mocks.HTTPClientMock{}
	mockHTTPClient.On("Do",
		mock.AnythingOfType("*http.Request"),
	).Return(&http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       ioutil.NopCloser(strings.NewReader("")),
	}, nil)

	workflowsClient := workflows.NewClient()
	workflowsClient.SetHTTPClient(mockHTTPClient)
	d.SetWorkflowsClient(workflowsClient)

	fs.On("GetRequest",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("time.Duration"),
		mock.AnythingOfType("string"),
	).Return(&model.Link{
		Uri: "GET",
	}, nil)

	fs.On("DeleteRequest",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("time.Duration"),
		mock.AnythingOfType("string"),
	).Return(&model.Link{
		Uri: "DELETE",
	}, nil)

	fs.On("UploadArtifact",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("int64"),
		mock.AnythingOfType("*io.LimitedReader"),
		mock.AnythingOfType("string"),
	).Return(nil)

	fs.On("Delete",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
	).Return(errors.New("unable to remove the file"))

	db.On("IsArtifactUnique",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string"),
	).Return(true, nil)

	multipartGenerateArtifact := &model.MultipartGenerateArtifactMsg{
		Name:                  "name",
		Description:           "description",
		DeviceTypesCompatible: []string{"Beagle Bone"},
		Type:                  "single_file",
		Args:                  "",
		Size:                  10,
		FileReader:            bytes.NewReader([]byte("123456790")),
	}

	ctx := context.Background()
	artifactID, err := d.GenerateArtifact(ctx, multipartGenerateArtifact)

	assert.Equal(t, artifactID, "")
	assert.Error(t, err)
	assert.EqualError(t, err, "unable to remove the file: failed to start workflow: generate_artifact")

	db.AssertExpectations(t)
	fs.AssertExpectations(t)
	mockHTTPClient.AssertExpectations(t)
}

func TestGenerateArtifactSuccessful(t *testing.T) {
	db := mocks.DataStore{}
	fs := &fs_mocks.FileStorage{}
	d := NewDeployments(&db, fs, ArtifactContentType)

	mockHTTPClient := &workflows_mocks.HTTPClientMock{}
	mockHTTPClient.On("Do",
		mock.MatchedBy(func(req *http.Request) bool {
			b, err := ioutil.ReadAll(req.Body)
			if err != nil {
				return false
			}
			multipartGenerateArtifact := &model.MultipartGenerateArtifactMsg{}
			err = json.Unmarshal(b, &multipartGenerateArtifact)
			if err != nil {
				return false
			}
			assert.Equal(t, "name", multipartGenerateArtifact.Name)
			assert.Equal(t, "description", multipartGenerateArtifact.Description)
			assert.Equal(t, int64(10), multipartGenerateArtifact.Size)
			assert.Len(t, multipartGenerateArtifact.DeviceTypesCompatible, 1)
			assert.Equal(t, "Beagle Bone", multipartGenerateArtifact.DeviceTypesCompatible[0])
			assert.Equal(t, "single_file", multipartGenerateArtifact.Type)
			assert.Equal(t, "args", multipartGenerateArtifact.Args)
			assert.Empty(t, multipartGenerateArtifact.TenantID)
			assert.NotEmpty(t, multipartGenerateArtifact.ArtifactID)
			assert.Equal(t, "GET", multipartGenerateArtifact.GetArtifactURI)
			assert.Equal(t, "DELETE", multipartGenerateArtifact.DeleteArtifactURI)
			return true
		}),
	).Return(&http.Response{
		StatusCode: http.StatusCreated,
		Body:       ioutil.NopCloser(strings.NewReader("")),
	}, nil)

	workflowsClient := workflows.NewClient()
	workflowsClient.SetHTTPClient(mockHTTPClient)
	d.SetWorkflowsClient(workflowsClient)

	fs.On("GetRequest",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("time.Duration"),
		mock.AnythingOfType("string"),
	).Return(&model.Link{
		Uri: "GET",
	}, nil)

	fs.On("DeleteRequest",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("time.Duration"),
		mock.AnythingOfType("string"),
	).Return(&model.Link{
		Uri: "DELETE",
	}, nil)

	fs.On("UploadArtifact",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("int64"),
		mock.AnythingOfType("*io.LimitedReader"),
		mock.AnythingOfType("string"),
	).Return(nil)

	db.On("IsArtifactUnique",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string"),
	).Return(true, nil)

	multipartGenerateArtifact := &model.MultipartGenerateArtifactMsg{
		Name:                  "name",
		Description:           "description",
		DeviceTypesCompatible: []string{"Beagle Bone"},
		Type:                  "single_file",
		Args:                  "args",
		Size:                  10,
		FileReader:            bytes.NewReader([]byte("123456790")),
	}

	ctx := context.Background()
	artifactID, err := d.GenerateArtifact(ctx, multipartGenerateArtifact)

	assert.NotEqual(t, artifactID, "")
	assert.Nil(t, err)

	db.AssertExpectations(t)
	fs.AssertExpectations(t)
	mockHTTPClient.AssertExpectations(t)
}

func TestGenerateArtifactSuccessfulWithTenant(t *testing.T) {
	db := mocks.DataStore{}
	fs := &fs_mocks.FileStorage{}
	d := NewDeployments(&db, fs, ArtifactContentType)

	mockHTTPClient := &workflows_mocks.HTTPClientMock{}
	mockHTTPClient.On("Do",
		mock.MatchedBy(func(req *http.Request) bool {
			b, err := ioutil.ReadAll(req.Body)
			if err != nil {
				return false
			}
			multipartGenerateArtifact := &model.MultipartGenerateArtifactMsg{}
			err = json.Unmarshal(b, &multipartGenerateArtifact)
			if err != nil {
				return false
			}
			assert.Equal(t, "name", multipartGenerateArtifact.Name)
			assert.Equal(t, "description", multipartGenerateArtifact.Description)
			assert.Equal(t, int64(10), multipartGenerateArtifact.Size)
			assert.Len(t, multipartGenerateArtifact.DeviceTypesCompatible, 1)
			assert.Equal(t, "Beagle Bone", multipartGenerateArtifact.DeviceTypesCompatible[0])
			assert.Equal(t, "single_file", multipartGenerateArtifact.Type)
			assert.Equal(t, "args", multipartGenerateArtifact.Args)
			assert.Equal(t, "tenant_id", multipartGenerateArtifact.TenantID)
			assert.NotEmpty(t, multipartGenerateArtifact.ArtifactID)
			assert.Equal(t, "GET", multipartGenerateArtifact.GetArtifactURI)
			assert.Equal(t, "DELETE", multipartGenerateArtifact.DeleteArtifactURI)
			return true
		}),
	).Return(&http.Response{
		StatusCode: http.StatusCreated,
		Body:       ioutil.NopCloser(strings.NewReader("")),
	}, nil)

	workflowsClient := workflows.NewClient()
	workflowsClient.SetHTTPClient(mockHTTPClient)
	d.SetWorkflowsClient(workflowsClient)

	fs.On("GetRequest",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("time.Duration"),
		mock.AnythingOfType("string"),
	).Return(&model.Link{
		Uri: "GET",
	}, nil)

	fs.On("DeleteRequest",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("time.Duration"),
		mock.AnythingOfType("string"),
	).Return(&model.Link{
		Uri: "DELETE",
	}, nil)

	fs.On("UploadArtifact",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("int64"),
		mock.AnythingOfType("*io.LimitedReader"),
		mock.AnythingOfType("string"),
	).Return(nil)

	db.On("IsArtifactUnique",
		h.ContextMatcher(),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string"),
	).Return(true, nil)

	multipartGenerateArtifact := &model.MultipartGenerateArtifactMsg{
		Name:                  "name",
		Description:           "description",
		DeviceTypesCompatible: []string{"Beagle Bone"},
		Type:                  "single_file",
		Args:                  "args",
		Size:                  10,
		FileReader:            bytes.NewReader([]byte("123456790")),
	}

	ctx := context.Background()
	identityObject := &identity.Identity{Tenant: "tenant_id"}
	ctxWithIdentity := identity.WithContext(ctx, identityObject)
	artifactID, err := d.GenerateArtifact(ctxWithIdentity, multipartGenerateArtifact)

	assert.NotEqual(t, artifactID, "")
	assert.Nil(t, err)

	db.AssertExpectations(t)
	fs.AssertExpectations(t)
	mockHTTPClient.AssertExpectations(t)
}
