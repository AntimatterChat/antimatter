// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package testutils

import (
	"net"
	"os"
	"testing"
	"time"
)

// RequireServicesEnv names the environment variable that turns a missing test service (MinIO, Inbucket, Redis) into a
// failure instead of a skip. CI sets it when it starts those services, so a broken service can't hide tests there.
const RequireServicesEnv = "AM_TEST_REQUIRE_SERVICES"

func serviceEndpoint(hostEnv, defaultHost, portEnv, defaultPort string) string {
	host := os.Getenv(hostEnv)
	if host == "" {
		host = defaultHost
	}
	port := os.Getenv(portEnv)
	if port == "" {
		port = defaultPort
	}
	return net.JoinHostPort(host, port)
}

// ServiceReachable tells whether something listens on endpoint.
func ServiceReachable(endpoint string) bool {
	conn, err := net.DialTimeout("tcp", endpoint, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// RedisEndpoint returns the Redis endpoint the tests use for the cache: CI_REDIS_HOST:CI_REDIS_PORT, by default
// localhost:6379, or redis:6379 in upstream's CI (IS_CI=true, where the services are other containers).
func RedisEndpoint() string {
	defaultHost := "localhost"
	if os.Getenv("IS_CI") == "true" && os.Getenv("CI_REDIS_HOST") == "" {
		defaultHost = "redis"
	}
	return serviceEndpoint("CI_REDIS_HOST", defaultHost, "CI_REDIS_PORT", "6379")
}

// UseRedisCache tells whether the tests should use the Redis test service as the cache: when it's reachable,
// or when AM_TEST_REQUIRE_SERVICES=1 (a missing Redis then fails the test instead of falling back to the
// in-memory cache).
func UseRedisCache() bool {
	return os.Getenv(RequireServicesEnv) == "1" || ServiceReachable(RedisEndpoint())
}

// SkipUnlessServiceReachable skips the test when nothing listens on endpoint, or fails it when
// AM_TEST_REQUIRE_SERVICES=1.
func SkipUnlessServiceReachable(tb testing.TB, name, endpoint string) {
	tb.Helper()
	conn, err := net.DialTimeout("tcp", endpoint, 2*time.Second)
	if err == nil {
		conn.Close()
		return
	}
	if os.Getenv(RequireServicesEnv) == "1" {
		tb.Fatalf("%s is not reachable at %s (%s=1): %v", name, endpoint, RequireServicesEnv, err)
	}
	tb.Skipf("%s is not reachable at %s, skipping: %v", name, endpoint, err)
}

// MinIOEndpoint returns the MinIO (S3) endpoint the tests use: CI_MINIO_HOST:CI_MINIO_PORT, localhost:9000 by
// default.
func MinIOEndpoint() string {
	return serviceEndpoint("CI_MINIO_HOST", "localhost", "CI_MINIO_PORT", "9000")
}

// SkipWithoutMinIO skips a test that needs the MinIO (S3) test service when it isn't running.
func SkipWithoutMinIO(tb testing.TB) {
	tb.Helper()
	SkipUnlessServiceReachable(tb, "MinIO", MinIOEndpoint())
}

// SkipWithoutInbucket skips a test that sends and reads mail through the Inbucket test service (SMTP on
// CI_INBUCKET_SMTP_PORT, 10025 by default, and its web API on CI_INBUCKET_PORT, 9001 by default) when it isn't
// running.
func SkipWithoutInbucket(tb testing.TB) {
	tb.Helper()
	SkipUnlessServiceReachable(tb, "Inbucket SMTP", serviceEndpoint("CI_INBUCKET_HOST", "localhost", "CI_INBUCKET_SMTP_PORT", "10025"))
	SkipUnlessServiceReachable(tb, "Inbucket", serviceEndpoint("CI_INBUCKET_HOST", "localhost", "CI_INBUCKET_PORT", "9001"))
}
