// Copyright 2021 AccelByte Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package healthcheck

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	commonblobgo "github.com/AccelByte/common-blob-go"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/AccelByte/iam-go-sdk"
	"github.com/aws/aws-sdk-go/aws/credentials"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/go-redis/redis/v8"
	"github.com/olivere/elastic"
	"github.com/sha1sum/aws_signing_client"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	pgdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const timeout = 5 * time.Second

//nolint: gosec
func TestElasticHealthCheck(t *testing.T) {
	assert.Error(t, ElasticHealthCheck(&elastic.Client{}, "", "", timeout)())

	clientSigner := v4.NewSigner(credentials.NewEnvCredentials())
	awsHTTPClient, err := aws_signing_client.New(clientSigner, nil, "es", "us-west-2")
	assert.Nil(t, err)

	awsHTTPClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	elasticClient, err := elastic.NewClient(
		elastic.SetURL(fmt.Sprintf("%s:%s", "http://localhost", "4571")),
		elastic.SetHttpClient(awsHTTPClient),
		elastic.SetSniff(false))

	assert.Nil(t, err)

	errElastic := ElasticHealthCheck(elasticClient, "http://localhost", "4571", timeout)()
	assert.Nil(t, errElastic, errElastic)
}

func TestIamHealthCheck(t *testing.T) {
	assert.Nil(t, IamHealthCheck(iam.NewMockClient())())
	assert.Error(t, IamHealthCheck(iam.Client(nil))())
}

func TestMongoHealthCheck(t *testing.T) {
	assert.Error(t, MongoHealthCheck(&mongo.Client{}, timeout)())

	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))

	assert.Nil(t, err)
	assert.Nil(t, MongoHealthCheck(client, timeout)())
}

func TestPostgresHealthCheck(t *testing.T) {
	assert.Error(t, PostgresHealthCheck(nil, timeout)())

	postgresArgs := fmt.Sprint("host=localhost port=5432 user=admin dbname=test password=admin")
	postgresClient, err := gorm.Open(pgdriver.Open(postgresArgs))

	assert.Nil(t, err)
	assert.Nil(t, PostgresHealthCheck(postgresClient, timeout)())
}

func TestPostgresV1HealthCheck(t *testing.T) {
	assert.Error(t, PostgresHealthCheckV1(nil, timeout)())

	postgresArgs := fmt.Sprint("host=localhost port=5432 user=admin dbname=test password=admin")
	postgresClient, err := gorm.Open(pgdriver.Open(postgresArgs))

	assert.Nil(t, err)
	assert.Nil(t, PostgresHealthCheck(postgresClient, timeout)())
}

func TestRedisHealthCheck(t *testing.T) {
	assert.Error(t, RedisHealthCheck(nil, timeout)())

	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "redispass",
	})
	assert.Nil(t, RedisHealthCheck(redisClient, timeout)())
}

func TestCloudStorageCheck(t *testing.T) {
	assert.Error(t, CloudStorageCheck(nil)())

	ctx := context.Background()

	cloudStorage, err := commonblobgo.NewCloudStorage(
		ctx,
		true,
		"aws",
		"data",
		"http://localhost:4572",
		"us-west-2",
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
		"{\"type\": \"service_account\", \"project_id\": \"my-project-id\"}",
		"false",
	)

	assert.Nil(t, err)

	if cloudStorage == nil || (reflect.ValueOf(cloudStorage).Kind() == reflect.Ptr &&
		reflect.ValueOf(cloudStorage).IsNil()) {
		assert.Fail(t, "empty instance of Cloud Storage")
	}

	err = cloudStorage.CreateBucket(context.Background(), "data/", 7)
	assert.Nil(t, err)

	awsSession, err := session.NewSession(&aws.Config{
		Region:           aws.String("us-west-2"),
		S3ForcePathStyle: aws.Bool(true),
	})
	assert.Nil(t, err)

	s3Client := s3.New(awsSession)
	assert.NotNil(t, s3Client)

	assert.Nil(t, CloudStorageCheck(cloudStorage)())
}
