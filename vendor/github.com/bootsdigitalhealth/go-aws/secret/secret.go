// Package secret provides helper methods for interacting with AWS Secrets Manager.
package secret

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-secretsmanager-caching-go/secretcache"
)

type cache interface {
	GetSecretString(secretID string) (string, error)
}

// Cache wraps secretcache.Cache for accessing AWS Secrets
type Cache struct {
	Cache cache
}

// New constructs a new Cache object with an embedded secretcache.Cache
func New() (*Cache, error) {
	client, err := GetClient()
	if err != nil {
		return nil, errors.New("Unable to generate custom client")
	}
	var innerCache *secretcache.Cache
	if client != nil {
		innerCache, err = secretcache.New(
			func(c *secretcache.Cache) { c.Client = client },
		)
	} else {
		innerCache, err = secretcache.New()
	}
	return &Cache{Cache: innerCache}, err
}

// GetSecretStringAsMap returns a secret from the embedded cache after converting it to a map
func (cache *Cache) GetSecretStringAsMap(secretID string) (map[string]interface{}, error) {
	secretString, err := cache.Cache.GetSecretString(secretID)
	if err != nil {
		return nil, errors.New("Unable to get secret " + secretID)
	}
	config := make(map[string]interface{})
	json.Unmarshal([]byte(secretString), &config)
	return config, nil
}

// Secret is used to get a secret value and return it as a map. Pass in the cache declared upstream.
//
// Deprecated: use New() and then Cache.GetSecretStringAsMap()
func Secret(cache *secretcache.Cache, name string) (map[string]interface{}, error) {
	secretString, err := cache.GetSecretString(name)
	if err != nil {
		return nil, errors.New("Unable to get secret " + name)
	}
	config := make(map[string]interface{})
	json.Unmarshal([]byte(secretString), &config)
	return config, nil
}

// Returns a custom client if appropriate
// Return nil to use the default client that the secretsmanager package creates
func GetClient() (*secretsmanager.SecretsManager, error){
	if useLocalStack, err := strconv.ParseBool(os.Getenv("AWS_USE_LOCALSTACK")); err == nil && useLocalStack == true {
		return GetLocalStackClient()
	}
	return nil, nil
}

func GetLocalStackClient() (*secretsmanager.SecretsManager, error){
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String("us-west-2"),
		Credentials:      credentials.NewStaticCredentials("test", "test", ""),
		Endpoint:         aws.String("http://aws-localstack.lh.local:4566"),
	})
	if err != nil {
		return nil, err
	}

	return secretsmanager.New(sess), nil
}
