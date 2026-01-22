/*
Copyright 2026 Shane Utt.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cache

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"
)

// -----------------------------------------------------------------------------
// Constants
// -----------------------------------------------------------------------------

// TimestampFormat is the RFC3339 format with milliseconds used for all timestamps
const TimestampFormat = time.RFC3339Nano

// CacheGCInterval is how often to check for and remove stale cache entries
const CacheGCInterval = 5 * time.Minute

// CacheMaxAge is the maximum age of a cache entry before it's considered stale
const CacheMaxAge = 24 * time.Hour

// CacheMaxSize is the maximum total size of all cached rules in bytes (100MB)
const CacheMaxSize = 100 * 1024 * 1024

// MaxHeaderSize is the maximum size of HTTP request headers (64KB)
const MaxHeaderSize = 64 * 1024

// MaxBodySize is the maximum size of HTTP request bodies (0 bytes - no body expected)
const MaxBodySize = 0

// -----------------------------------------------------------------------------
// API Response Types
// -----------------------------------------------------------------------------

// LatestResponse contains metadata about the latest ruleset version
type LatestResponse struct {
	UUID      string `json:"uuid"`
	Timestamp string `json:"timestamp"`
}

// -----------------------------------------------------------------------------
// RuleSetCacheServer
// -----------------------------------------------------------------------------

// RuleSetCacheServer provides HTTP endpoints for accessing cached rulesets
type RuleSetCacheServer struct {
	cache  *RuleSetCache
	srv    *http.Server
	logger logr.Logger
	gc     GarbageCollectionConfig
}

// NewServer creates a new RuleSetCacheServer instance.
func NewServer(cache *RuleSetCache, addr string, logger logr.Logger, gc *GarbageCollectionConfig) *RuleSetCacheServer {
	gcConfig := DefaultGC()
	if gc != nil {
		gcConfig = *gc
	}

	s := &RuleSetCacheServer{
		cache:  cache,
		logger: logger,
		gc:     gcConfig,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/rules/", s.handleRules)

	s.srv = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    MaxHeaderSize,
	}

	return s
}

// Start the cache server.
func (s *RuleSetCacheServer) Start(ctx context.Context) error {
	go s.rungc(ctx)

	errChan := make(chan error, 1)
	go func() {
		s.logger.Info("Starting ruleset cache server", "addr", s.srv.Addr)
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case <-ctx.Done():
		s.logger.Info("Shutting down ruleset cache server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.srv.Shutdown(shutdownCtx)
	case err := <-errChan:
		return err
	}
}

// NeedLeaderElection implements the LeaderElectionRunnable interface.
func (s *RuleSetCacheServer) NeedLeaderElection() bool {
	return false
}

// -----------------------------------------------------------------------------
// RuleSetCacheServer - Handlers
// -----------------------------------------------------------------------------

func (s *RuleSetCacheServer) handleRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/rules/")
	if path == "" {
		http.Error(w, "Instance name required", http.StatusBadRequest)
		return
	}

	if strings.HasSuffix(path, "/latest") {
		instance := strings.TrimSuffix(path, "/latest")
		s.handleLatest(w, r, instance)
		return
	}

	s.handleGetRules(w, r, path)
}

func (s *RuleSetCacheServer) handleLatest(w http.ResponseWriter, _ *http.Request, instance string) {
	entry, ok := s.cache.Get(instance)
	if !ok {
		http.Error(w, "Instance not found", http.StatusNotFound)
		return
	}

	response := LatestResponse{
		UUID:      entry.UUID,
		Timestamp: entry.Timestamp.Format(TimestampFormat),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error(err, "Failed to encode latest response")
	}
}

func (s *RuleSetCacheServer) handleGetRules(w http.ResponseWriter, _ *http.Request, instance string) {
	entry, ok := s.cache.Get(instance)
	if !ok {
		http.Error(w, "Instance not found", http.StatusNotFound)
		return
	}

	s.logger.Info("Serving rules from cache", "instance", instance, "uuid", entry.UUID, "availableKeys", s.cache.ListKeys(), "cacheSizeBytes", s.cache.TotalSize())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(entry); err != nil {
		s.logger.Error(err, "Failed to encode rules response")
	}
}

// -----------------------------------------------------------------------------
// RuleSetCacheServer - Garbage Collection
// -----------------------------------------------------------------------------

// GarbageCollectionConfig is the GC config for the RuleSetCacheServer.
type GarbageCollectionConfig struct {
	// GCInterval is how often to check for and remove stale cache entries.
	GCInterval time.Duration

	// MaxAge is the maximum age of a cache entry before it's considered stale.
	MaxAge time.Duration

	// MaxSize is the maximum total size of all cached rules in bytes.
	MaxSize int
}

// DefaultGC returns the default garbage collection configuration.
func DefaultGC() GarbageCollectionConfig {
	return GarbageCollectionConfig{
		GCInterval: CacheGCInterval,
		MaxAge:     CacheMaxAge,
		MaxSize:    CacheMaxSize,
	}
}

// rungc periodically removes stale cache entries using two strategies:
// 1. Age-based: entries older than MaxAge (except latest)
// 2. Size-based: oldest entries when cache exceeds MaxSize (except latest)
func (s *RuleSetCacheServer) rungc(ctx context.Context) {
	ticker := time.NewTicker(s.gc.GCInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			prunedByAge := s.cache.Prune(s.gc.MaxAge)
			if prunedByAge > 0 {
				s.logger.Info("Pruned stale cache entries by age", "count", prunedByAge, "maxAge", s.gc.MaxAge)
			}

			currentSize := s.cache.TotalSize()
			if currentSize > s.gc.MaxSize {
				prunedBySize := s.cache.PruneBySize(s.gc.MaxSize)
				if prunedBySize > 0 {
					s.logger.Info("Pruned cache entries by size", "count", prunedBySize, "maxSize", s.gc.MaxSize, "currentSize", s.cache.TotalSize())
				}

				finalSize := s.cache.TotalSize()
				if finalSize > s.gc.MaxSize {
					s.logger.Error(errors.New("cache size exceeds maximum"), "CRITICAL: Cache size exceeds maximum even after pruning - latest entry is too large", "currentSize", finalSize, "maxSize", s.gc.MaxSize, "overage", finalSize-s.gc.MaxSize)
				}
			}
		}
	}
}
