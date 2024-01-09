// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package dataflowaccessor

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/dataflow/apiv1beta3/dataflowpb"
	dataflowclient "github.com/GoogleCloudPlatform/spanner-migration-tool/accessors/clients/dataflow"
	storageacc "github.com/GoogleCloudPlatform/spanner-migration-tool/accessors/storage"
	"github.com/GoogleCloudPlatform/spanner-migration-tool/logger"
	"github.com/googleapis/gax-go/v2"
)

type DataflowAccessor interface {
	LaunchFlexTemplate(ctx context.Context, req *dataflowpb.LaunchFlexTemplateRequest, opts ...gax.CallOption) (*dataflowpb.LaunchFlexTemplateResponse, error)
}

type DataflowAccessorImpl struct{}

func (dfA *DataflowAccessorImpl) LaunchFlexTemplate(ctx context.Context, req *dataflowpb.LaunchFlexTemplateRequest, opts ...gax.CallOption) (*dataflowpb.LaunchFlexTemplateResponse, error) {
	dfClient, err := dataflowclient.GetOrCreateClient(ctx)
	if err != nil {
		return nil, err
	}
	respDf, err := dfClient.LaunchFlexTemplate(ctx, req)
	if err != nil {
		logger.Log.Error(fmt.Sprintf("flexTemplateRequest: %+v\n", req))
		return nil, fmt.Errorf("error launching dataflow template: %v", err)
	}
	return respDf, nil
}

func UnmarshalDataflowTuningConfig(ctx context.Context, filePath string) (DataflowTuningConfig, error) {
	jsonStr, err := storageacc.ReadAnyFile(ctx, filePath)
	if err != nil {
		return DataflowTuningConfig{}, err
	}
	tuningCfg := DataflowTuningConfig{}
	err = json.Unmarshal([]byte(jsonStr), &tuningCfg)
	if err != nil {
		return DataflowTuningConfig{}, err
	}
	return tuningCfg, nil
}
