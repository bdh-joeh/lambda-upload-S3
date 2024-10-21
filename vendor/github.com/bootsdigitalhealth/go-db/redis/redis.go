// Package redis provides a convenient way to get a Redis client
package redis

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redis/redis"
	"strconv"
)

// Client wraps redis.Client
type Client struct {
	*redis.Client
}

// GetSession provides a Session given a token
func (c *Client) GetSession(token string) (Session, error) {
	s := Session{}
	data, err := c.Get(token).Result()
	if err == redis.Nil {
		return s, nil
	} else if err != nil {
		return s, err
	} else {
		err = json.Unmarshal([]byte(data), &s)
		if err != nil {
			return Session{}, err
		}
		s.Token = token
		return s, nil
	}
}

// GetStringValue returns the raw string stored with a specified key.
// The second return value will be false if the key did not exist
func (c *Client) GetStringValue(key string) (string, bool, error) {
	data, err := c.Get(key).Result()
	if err == redis.Nil {
		return "", false, nil
	} else if err != nil {
		return "", false, err
	} else {
		return data, true, nil
	}
}

// GetByteArrayValue returns the string value stored with a specified key, but as a byte array
func (c *Client) GetByteArrayValue(key string) ([]byte, bool, error) {
	stringValue, ok, err := c.GetStringValue(key)
	return []byte(stringValue), ok, err
}

// BuildCacheKey returns a cache key string based on a prefix plus passed params
func BuildCacheKey(prefix string, params map[string]string) string {
	cacheKey := prefix
	for k, v := range params {
		cacheKey += fmt.Sprintf(":%s:%s", k, v)
	}
	return cacheKey
}

// Session represents the value of a Session in the Redis db
type Session struct {
	UserID  int64             `json:"user_id"`
	Roles   map[string]string `json:"roles"`
	Created int64             `json:"created"`
	Timeout int               `json:"timeout"`
	Token   string
}

// NewClient provides a redis client. Pass in a config map and the name of the db.
//
// db names are:
// - sessions_db
// - routes_db
// - locks_db
// - service_cache_db
func NewClient(config map[string]interface{}, dbName string) (*Client, error) {
	db, err := strconv.Atoi(config[dbName].(string))
	if err != nil {
		return &Client{}, errors.New("Failed to get session db id")
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr:     config["host"].(string) + ":" + config["port"].(string),
		Password: "",
		DB:       db,
	})
	return &Client{redisClient}, nil
}

// Redis is used to get a Redis client. Pass in a config map and the name of the db.
//
// Deprecated. Use NewClient().
func Redis(config map[string]interface{}, dbName string) (*redis.Client, error) {
	db, err := strconv.Atoi(config[dbName].(string))
	if err != nil {
		return &redis.Client{}, errors.New("Failed to get session db id")
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr:     config["host"].(string) + ":" + config["port"].(string),
		Password: "",
		DB:       db,
	})
	return redisClient, nil
}
