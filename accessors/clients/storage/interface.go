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
package storageclient

import (
	"context"
	"io"

	"cloud.google.com/go/storage"
)

type StorageClient interface {
	Bucket(name string) BucketHandle
}

type BucketHandle interface {
	Create(ctx context.Context, projectID string, attrs *storage.BucketAttrs) (err error)
	Update(ctx context.Context, uattrs storage.BucketAttrsToUpdate) (attrs *storage.BucketAttrs, err error)
	Object(name string) ObjectHandle
}

type ObjectHandle interface {
	NewWriter(ctx context.Context) io.WriteCloser
	NewReader(ctx context.Context) (io.ReadCloser, error)
}

type StorageClientImpl struct {
	client *storage.Client
}

func NewStorageClientImpl(ctx context.Context) (*StorageClientImpl, error) {
	c, err := GetOrCreateClient(ctx)
	if err != nil {
		return nil, err
	}
	return &StorageClientImpl{client: c}, nil
}

func (c *StorageClientImpl) Bucket(name string) BucketHandle {
	return &BucketHandleImpl{bucketHandle: c.client.Bucket(name)}
}

type BucketHandleImpl struct {
	bucketHandle *storage.BucketHandle
}

func (b *BucketHandleImpl) Create(ctx context.Context, projectID string, attrs *storage.BucketAttrs) (err error) {
	return b.bucketHandle.Create(ctx, projectID, attrs)
}

func (b *BucketHandleImpl) Update(ctx context.Context, uattrs storage.BucketAttrsToUpdate) (attrs *storage.BucketAttrs, err error) {
	return b.bucketHandle.Update(ctx, uattrs)
}

func (b *BucketHandleImpl) Object(name string) ObjectHandle {
	return &ObjectHandleImpl{objectHandle: b.bucketHandle.Object(name)}
}

type ObjectHandleImpl struct {
	objectHandle *storage.ObjectHandle
}

func (o *ObjectHandleImpl) NewWriter(ctx context.Context) io.WriteCloser {
	return o.objectHandle.NewWriter(ctx)
}

func (o *ObjectHandleImpl) NewReader(ctx context.Context) (io.ReadCloser, error) {
	return o.objectHandle.NewReader(ctx)
}
