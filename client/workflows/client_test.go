// Copyright 2019 Northern.tech AS
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

package workflows

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	workflow_mocks "github.com/mendersoftware/deployments/client/workflows/mocks"
	"github.com/mendersoftware/deployments/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGenerateArtifactFails(t *testing.T) {
	mockHTTPClient := &workflow_mocks.HTTPClientMock{}
	mockHTTPClient.On("Do",
		mock.AnythingOfType("*http.Request"),
	).Return(&http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       ioutil.NopCloser(strings.NewReader("")),
	}, nil)

	workflowsClient := NewClient()
	workflowsClient.SetHTTPClient(mockHTTPClient)

	multipartGenerateArtifact := &model.MultipartGenerateArtifactMsg{
		Name:                  "name",
		Description:           "description",
		DeviceTypesCompatible: []string{"Beagle Bone"},
		Type:                  "single_file",
		Args:                  "",
		Size:                  10,
		TenantID:              "tenant_id",
		ArtifactID:            "artifact_id",
		FileReader:            bytes.NewReader([]byte("123456790")),
	}

	ctx := context.Background()
	err := workflowsClient.StartGenerateArtifact(ctx, multipartGenerateArtifact)
	assert.Error(t, err)
	assert.EqualError(t, err, "failed to start workflow: generate_artifact")

	mockHTTPClient.AssertExpectations(t)
}

func TestGenerateArtifactSuccessful(t *testing.T) {
	mockHTTPClient := &workflow_mocks.HTTPClientMock{}
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
			assert.Equal(t, "artifact_id", multipartGenerateArtifact.ArtifactID)
			return true
		}),
	).Return(&http.Response{
		StatusCode: http.StatusCreated,
		Body:       ioutil.NopCloser(strings.NewReader("")),
	}, nil)

	workflowsClient := NewClient()
	workflowsClient.SetHTTPClient(mockHTTPClient)

	multipartGenerateArtifact := &model.MultipartGenerateArtifactMsg{
		Name:                  "name",
		Description:           "description",
		DeviceTypesCompatible: []string{"Beagle Bone"},
		Type:                  "single_file",
		Args:                  "args",
		Size:                  10,
		TenantID:              "tenant_id",
		ArtifactID:            "artifact_id",
		FileReader:            bytes.NewReader([]byte("123456790")),
	}

	ctx := context.Background()
	err := workflowsClient.StartGenerateArtifact(ctx, multipartGenerateArtifact)
	assert.Nil(t, err)

	mockHTTPClient.AssertExpectations(t)
}
