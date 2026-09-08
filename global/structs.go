package global

import "time"

// Описание структуры конфигурации
//type Cfg struct {
//	Minioclientcfg struct {
//		Endpoints []string `yaml:"endpoints"`
//		Port      string   `yaml:"port"`
//		Acesskey  string   `yaml:"acesskey"`
//		Secretkey string   `yaml:"secretkey"`
//	} `yaml:"minioclientcfg"`
//}

type Cfg struct {
	Connections []struct {
		Name      string   `yaml:"name"`
		Endpoints []string `yaml:"endpoints"`
		Port      string   `yaml:"port"`
		Acesskey  string   `yaml:"acesskey"`
		Secretkey string   `yaml:"secretkey"`
	} `yaml:"connections"`
}

// Структура обхекта бакета
type BucketObject struct {
	Etag         string      `json:"etag"`
	Name         string      `json:"name"`
	LastModified time.Time   `json:"lastModified"`
	Size         int         `json:"size"`
	ContentType  string      `json:"contentType"`
	Expires      time.Time   `json:"expires"`
	Metadata     interface{} `json:"metadata"`
	UserTagCount int         `json:"UserTagCount"`
	Owner        struct {
		Owner struct {
			Space string `json:"Space"`
			Local string `json:"Local"`
		} `json:"owner"`
		Name string `json:"name"`
		ID   string `json:"id"`
	} `json:"Owner"`
	Grant             interface{} `json:"Grant"`
	StorageClass      string      `json:"storageClass"`
	IsLatest          bool        `json:"IsLatest"`
	IsDeleteMarker    bool        `json:"IsDeleteMarker"`
	VersionID         string      `json:"VersionID"`
	ReplicationStatus string      `json:"ReplicationStatus"`
	ReplicationReady  bool        `json:"ReplicationReady"`
	Expiration        time.Time   `json:"Expiration"`
	ExpirationRuleID  string      `json:"ExpirationRuleID"`
	NumVersions       int         `json:"NumVersions"`
	Restore           interface{} `json:"Restore"`
	ChecksumCRC32     string      `json:"ChecksumCRC32"`
	ChecksumCRC32C    string      `json:"ChecksumCRC32C"`
	ChecksumSHA1      string      `json:"ChecksumSHA1"`
	ChecksumSHA256    string      `json:"ChecksumSHA256"`
	ChecksumCRC64NVME string      `json:"ChecksumCRC64NVME"`
	ChecksumMode      string      `json:"ChecksumMode"`
	Internal          interface{} `json:"Internal"`
}

// Структура статистики бакета
type BucketStats struct {
	Bucket            string            `json:"bucket"`
	Tenant            string            `json:"tenant"`
	Versioning        string            `json:"versioning"`
	Zonegroup         string            `json:"zonegroup"`
	PlacementRule     string            `json:"placement_rule"`
	ExplicitPlacement ExplicitPlacement `json:"explicit_placement"`
	ID                string            `json:"id"`
	Marker            string            `json:"marker"`
	IndexType         string            `json:"index_type"`
	IndexGeneration   int64             `json:"index_generation"`
	NumShards         int64             `json:"num_shards"`
	ObjectLockEnabled bool              `json:"object_lock_enabled"`
	MfaEnabled        bool              `json:"mfa_enabled"`
	Owner             string            `json:"owner"`
	Ver               string            `json:"ver"`
	MasterVer         string            `json:"master_ver"`
	Mtime             string            `json:"mtime"`
	CreationTime      string            `json:"creation_time"`
	MaxMarker         string            `json:"max_marker"`
	Usage             Usage             `json:"usage"`
	BucketQuota       BucketQuota       `json:"bucket_quota"`
}

type BucketQuota struct {
	Enabled    bool  `json:"enabled"`
	CheckOnRaw bool  `json:"check_on_raw"`
	MaxSize    int64 `json:"max_size"`
	MaxSizeKB  int64 `json:"max_size_kb"`
	MaxObjects int64 `json:"max_objects"`
}

type ExplicitPlacement struct {
	DataPool      string `json:"data_pool"`
	DataExtraPool string `json:"data_extra_pool"`
	IndexPool     string `json:"index_pool"`
}

type Usage struct {
	RgwMain RgwMain `json:"rgw.main"`
}

type RgwMain struct {
	Size           int64 `json:"size"`
	SizeActual     int64 `json:"size_actual"`
	SizeUtilized   int64 `json:"size_utilized"`
	SizeKB         int64 `json:"size_kb"`
	SizeKBActual   int64 `json:"size_kb_actual"`
	SizeKBUtilized int64 `json:"size_kb_utilized"`
	NumObjects     int64 `json:"num_objects"`
}

// Описание структуры объекта OLH индексной записи
type OlhObjectIndexRecord struct {
	Entry struct {
		DeleteMarker bool `json:"delete_marker"`
		Epoch        int  `json:"epoch"`
		Exists       bool `json:"exists"`
		Key          struct {
			Instance string `json:"instance"`
			Name     string `json:"name"`
		} `json:"key"`
		PendingLog     []interface{} `json:"pending_log"`
		PendingRemoval bool          `json:"pending_removal"`
		Tag            string        `json:"tag"`
	} `json:"entry"`
	Idx  string `json:"idx"`
	Type string `json:"type"`
}
