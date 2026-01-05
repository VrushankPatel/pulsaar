# API Reference

## PulsaarAgent Service

The PulsaarAgent service provides read-only file operations via gRPC.

### RPC Methods

#### ListDirectory

Lists files and directories in a given path.

**Request: ListRequest**

- `path` (string): The directory path to list
- `allowed_roots` (repeated string): List of allowed root paths for security

**Response: ListResponse**

- `entries` (repeated FileInfo): List of file/directory information

#### Stat

Gets file or directory statistics.

**Request: StatRequest**

- `path` (string): The file/directory path
- `allowed_roots` (repeated string): Allowed roots

**Response: StatResponse**

- `info` (FileInfo): File information

#### ReadFile

Reads a portion of a file.

**Request: ReadRequest**

- `path` (string): File path
- `offset` (int64): Byte offset to start reading
- `length` (int64): Number of bytes to read
- `allowed_roots` (repeated string): Allowed roots

**Response: ReadResponse**

- `data` (bytes): File data
- `eof` (bool): True if end of file reached

#### StreamFile

Streams a file in chunks.

**Request: StreamRequest**

- `path` (string): File path
- `chunk_size` (int64): Size of each chunk
- `allowed_roots` (repeated string): Allowed roots

**Response: stream ReadResponse**

- Stream of ReadResponse messages

#### Health

Checks agent health.

**Request: google.protobuf.Empty**

**Response: HealthResponse**

- `ready` (bool): If agent is ready
- `version` (string): Agent version
- `status_message` (string): Status message

### Messages

#### FileInfo

- `name` (string): File name
- `is_dir` (bool): True if directory
- `size_bytes` (int64): Size in bytes
- `mode` (string): File mode
- `mtime` (google.protobuf.Timestamp): Modification time

#### ListRequest

- `path` (string)
- `allowed_roots` (repeated string)

#### ListResponse

- `entries` (repeated FileInfo)

#### StatRequest

- `path` (string)
- `allowed_roots` (repeated string)

#### StatResponse

- `info` (FileInfo)

#### ReadRequest

- `path` (string)
- `offset` (int64)
- `length` (int64)
- `allowed_roots` (repeated string)

#### ReadResponse

- `data` (bytes)
- `eof` (bool)

#### StreamRequest

- `path` (string)
- `chunk_size` (int64)
- `allowed_roots` (repeated string)

#### HealthResponse

- `ready` (bool)
- `version` (string)
- `status_message` (string)