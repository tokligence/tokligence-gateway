# Google Gemini API Endpoints - TODO

This document tracks Google Gemini API endpoints and features that are not yet supported by the Tokligence Gateway.

**Last Updated:** 2025-11-19
**Research Focus:** Models and features released in 2024-2025

---

## ✅ Currently Supported

- ✅ `POST /v1beta/models/{model}:generateContent` - Standard content generation
- ✅ `POST /v1beta/models/{model}:streamGenerateContent` - SSE streaming generation
- ✅ `POST /v1beta/models/{model}:countTokens` - Token counting
- ✅ `GET /v1beta/models` - List available models
- ✅ `GET /v1beta/models/{model}` - Get model metadata
- ✅ `POST /v1beta/openai/chat/completions` - OpenAI-compatible endpoint (non-streaming)
- ✅ `POST /v1beta/openai/chat/completions` (streaming) - OpenAI-compatible streaming

---

## 🚧 Missing Endpoints

### 1. Embeddings API

**Priority:** High
**Release Date:** Generally available in 2024

#### Endpoints
- [ ] `POST /v1beta/models/gemini-embedding-001:embedContent`
  - Generate text embeddings for single input
  - Support for `task_type` parameter (SEMANTIC_SIMILARITY, CLASSIFICATION, CLUSTERING, etc.)
  - Support for `output_dimensionality` (128-3072 dimensions)
  - Input limit: 2,048 tokens

- [ ] `POST /v1beta/models/gemini-embedding-001:batchEmbedContents`
  - Batch embedding generation for multiple inputs
  - 50% cost reduction compared to single embeddings
  - Higher throughput for non-latency-critical applications

**Models:**
- `gemini-embedding-001` (stable production model)
- `embedding-gecko-001` (legacy, deprecated Oct 2025)
- `text-embedding-004` (latest version)

**Key Features:**
- 8 specialized task types for optimization
- Flexible dimensionality (128-3072)
- Multimodal embeddings (text + images)

**Implementation Notes:**
- Need to add dedicated embedding adapter
- Support task_type parameter for optimization
- Add dimension configuration
- Consider adding batch processing support

---

### 2. Files API (Media Upload)

**Priority:** High
**Release Date:** Available since 2024

#### Endpoints
- [ ] `POST /upload/v1beta/files`
  - Resumable upload protocol
  - Support for audio, images, videos, documents
  - Max file size: 2 GB per file
  - Requires `X-Goog-Upload-*` headers

- [ ] `GET /v1beta/files/{name}`
  - Get file metadata
  - Verify successful upload
  - Retrieve file information

- [ ] `GET /v1beta/files`
  - List all uploaded files
  - Support pagination via `pageSize`
  - Project-level file management

- [ ] `DELETE /v1beta/files/{name}`
  - Manual file deletion
  - Auto-deletion after 48 hours
  - Storage management

**Storage Limits:**
- 20 GB per project
- 48-hour file retention
- No cost for Files API

**Use Cases:**
- Large media files (>20MB)
- Reusable files across multiple requests
- Video/audio processing
- PDF and document analysis

**Implementation Notes:**
- Implement resumable upload protocol
- Add file storage tracking
- Support multipart uploads
- Add cleanup for expired files

---

### 3. Context Caching API

**Priority:** Medium
**Release Date:** May 2024 (explicit), May 2025 (implicit)

#### Endpoints
- [ ] `POST /v1beta/cachedContents`
  - Create cached content
  - Store large contexts for reuse
  - 75-90% cost savings on repeated context

- [ ] `GET /v1beta/cachedContents/{name}`
  - Retrieve cache metadata
  - Check cache expiration
  - Verify cache availability

- [ ] `PATCH /v1beta/cachedContents/{name}`
  - Update cache TTL
  - Extend cache lifetime
  - Modify cache parameters

- [ ] `DELETE /v1beta/cachedContents/{name}`
  - Manual cache deletion
  - Resource cleanup
  - Cache management

**Features:**
- Explicit caching: 90% discount (Gemini 2.5), 75% discount (Gemini 2.0)
- Implicit caching: Automatic (enabled by default on Gemini 2.5)
- Minimum tokens: 2,048 (Flash), 4,096 (Pro)
- Cache TTL: Configurable

**Use Cases:**
- Large document analysis
- Repeated queries on same context
- Code repositories analysis
- Long conversations with context preservation

**Implementation Notes:**
- Add cache storage backend
- Implement cache key generation
- Support TTL management
- Add cache hit/miss metrics

---

### 4. Batch Processing API

**Priority:** Medium
**Release Date:** 2024

#### Endpoints
- [ ] `POST /v1beta/models/{model}:batchGenerateContent`
  - Asynchronous batch processing
  - 50% cost reduction
  - Large-scale request handling

- [ ] `GET /v1beta/batches/{name}`
  - Check batch job status
  - Retrieve batch results
  - Monitor progress

- [ ] `POST /v1beta/batches/{name}:cancel`
  - Cancel running batch job
  - Resource management
  - Cost control

**Features:**
- 50% cost reduction vs. real-time API
- Asynchronous processing
- Suitable for non-latency-critical tasks
- Support for embeddings in batch mode

**Use Cases:**
- Large dataset processing
- Bulk content generation
- Offline analysis
- Cost-optimized workloads

**Implementation Notes:**
- Add job queue system
- Implement async job processing
- Add status tracking
- Support result retrieval

---

### 5. Bidirectional Streaming (WebSocket)

**Priority:** Low
**Release Date:** Available in 2024

#### Endpoints
- [ ] `WS /v1beta/models/{model}:bidiGenerateContent`
  - Real-time bidirectional streaming
  - Stateful conversations
  - Low-latency interactions

**Features:**
- WebSocket-based communication
- Stateful multi-turn conversations
- Real-time audio/video streaming
- Interactive applications

**Use Cases:**
- Voice assistants
- Real-time translation
- Interactive chatbots
- Live transcription

**Implementation Notes:**
- Add WebSocket support to gateway
- Implement connection pooling
- Support state management
- Add reconnection logic

---

### 6. Specialized Generation Models

**Priority:** Low to Medium
**Release Date:** Various 2024-2025

#### Image Generation (Imagen)
- [ ] Imagen 4 Ultra endpoint
- [ ] Imagen 4 Standard endpoint
- [ ] Imagen 4 Fast endpoint
- [ ] Gemini 2.5 Image Preview

**Features:**
- Native image generation
- Multiple quality tiers
- Fast generation mode

#### Video Generation (Veo)
- [ ] Veo video generation endpoint
- [ ] Video editing capabilities

#### Music Generation (Lyria)
- [ ] Music generation endpoint
- [ ] Audio synthesis

**Implementation Notes:**
- Separate adapters for each media type
- Support for specialized parameters
- Handle binary response formats
- Add media validation

---

### 7. Advanced Multimodal Features

**Priority:** Medium
**Release Date:** 2024-2025

#### Inline Media Support
- [ ] Base64 image encoding (<20MB)
- [ ] Inline audio processing
- [ ] Inline video processing
- [ ] PDF processing

#### File API Integration
- [ ] Media file references in prompts
- [ ] Cross-request file reuse
- [ ] Multiple file types in single request

**Supported Media Types:**
- Images: PNG, JPEG, WEBP, HEIC, HEIF
- Audio: WAV, MP3, AIFF, AAC, OGG, FLAC
- Video: MP4, MPEG, MOV, AVI, FLV, MPG, WEBM, WMV, 3GPP
- Documents: PDF, spreadsheets

**Implementation Notes:**
- Add media type validation
- Support inline data encoding
- Integrate with Files API
- Add size limit checks

---

### 8. Model-Specific Features

**Priority:** Low
**Release Date:** 2024-2025

#### Medical Models (MedGemma)
- [ ] MedGemma-4b-it (image-text-to-text)
- [ ] MedGemma-27b-text-it (medical text generation)

#### Specialized Models
- [ ] TimesFM-2.5-200m (time series forecasting)
- [ ] EmbeddingGemma-300m (specialized embeddings)
- [ ] VaultGemma-1b (security-focused)
- [ ] Gemma-3n variants (LiteRT on-device)

**Implementation Notes:**
- Model-specific parameter support
- Specialized response formats
- Domain-specific validation

---

## 📊 Priority Matrix

| Category | Priority | Complexity | Impact | Estimated Effort |
|----------|----------|------------|--------|------------------|
| Embeddings API | High | Medium | High | 2-3 days |
| Files API | High | High | High | 1 week |
| Context Caching | Medium | High | Medium | 1 week |
| Batch API | Medium | High | Medium | 1-2 weeks |
| Bidirectional Streaming | Low | Very High | Medium | 2 weeks |
| Specialized Models | Low | Low | Low | Variable |

---

## 🎯 Recommended Implementation Order

1. **Phase 1: Embeddings** (High Priority, Medium Complexity)
   - Single embedding generation
   - Batch embedding generation
   - Task type support

2. **Phase 2: Files API** (High Priority, High Complexity)
   - File upload (resumable)
   - File listing and metadata
   - File deletion and cleanup

3. **Phase 3: Context Caching** (Medium Priority, High Complexity)
   - Cache creation and storage
   - Cache retrieval and validation
   - TTL management

4. **Phase 4: Batch Processing** (Medium Priority, High Complexity)
   - Async job submission
   - Status tracking
   - Result retrieval

5. **Phase 5: Advanced Features** (Low Priority, Variable Complexity)
   - Bidirectional streaming
   - Specialized models
   - Media generation

---

## 📝 Implementation Notes

### General Considerations

1. **Authentication:**
   - All endpoints use `x-goog-api-key` header
   - Gateway should handle authentication centrally
   - Support for both query param and header formats

2. **Rate Limiting:**
   - Implement per-endpoint rate limits
   - Track quota usage
   - Handle 429 responses gracefully

3. **Error Handling:**
   - Standardize error response format
   - Map Gemini errors to gateway errors
   - Provide helpful error messages

4. **Monitoring:**
   - Add endpoint-specific metrics
   - Track usage by feature
   - Monitor cost attribution

5. **Documentation:**
   - Update integration guide for each feature
   - Add usage examples
   - Document cost implications

---

## 🔗 References

- [Gemini API Documentation](https://ai.google.dev/gemini-api/docs)
- [Embeddings Guide](https://ai.google.dev/gemini-api/docs/embeddings)
- [Files API Guide](https://ai.google.dev/gemini-api/docs/files)
- [Context Caching Guide](https://ai.google.dev/gemini-api/docs/caching)
- [Batch API Guide](https://apidog.com/blog/gemini-api-batch-mode/)
- [Google Hugging Face Models](https://huggingface.co/google)

---

## 📅 Version History

- **2025-11-19:** Initial TODO list created based on 2024-2025 features
- Features researched from Google AI documentation and Hugging Face

---

**Note:** This list focuses on features released or updated in 2024-2025. Legacy features and deprecated endpoints are intentionally excluded.
