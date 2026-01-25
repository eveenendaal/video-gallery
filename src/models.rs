use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Category {
    pub name: String,
    pub stub: String,
    pub galleries: Vec<Gallery>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Gallery {
    pub name: String,
    pub category: String,
    pub stub: String,
    pub videos: Vec<Video>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Video {
    pub name: String,
    pub category: String,
    pub gallery: String,
    pub url: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub thumbnail: Option<String>,
    pub video_path: String,
    pub thumbnail_path: String,
}

#[derive(Debug, Deserialize)]
pub struct ClearThumbnailRequest {
    #[serde(rename = "thumbnailPath")]
    pub thumbnail_path: String,
}

#[derive(Debug, Deserialize)]
pub struct BulkGenerateRequest {
    #[serde(rename = "videoPaths")]
    pub video_paths: Vec<String>,
    #[serde(rename = "timeMs")]
    pub time_ms: i32,
    #[serde(rename = "maxParallel")]
    pub max_parallel: Option<usize>,
}

#[derive(Debug, Deserialize)]
pub struct BulkClearRequest {
    #[serde(rename = "thumbnailPaths")]
    pub thumbnail_paths: Vec<String>,
}

#[derive(Debug, Serialize)]
pub struct MoviePosterResult {
    pub title: String,
    pub year: Option<i32>,
    #[serde(rename = "posterPath")]
    pub poster_path: Option<String>,
    #[serde(rename = "thumbnailUrl")]
    pub thumbnail_url: Option<String>,
}

#[derive(Debug, Serialize)]
pub struct ProgressUpdate {
    pub step: String,
    pub progress: i32,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}
