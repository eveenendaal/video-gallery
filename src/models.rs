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
    #[serde(skip_serializing)]
    pub stub: String,
    pub videos: Vec<Video>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Video {
    pub name: String,
    #[serde(skip_serializing)]
    pub category: String,
    #[serde(skip_serializing)]
    pub gallery: String,
    pub url: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub thumbnail: Option<String>,
    #[serde(skip)]
    pub video_path: String,
    #[serde(skip)]
    pub thumbnail_path: String,
}

#[derive(Debug, Serialize)]
pub struct Index {
    pub categories: Vec<Category>,
}

#[derive(Debug, Serialize)]
pub struct Admin {
    pub categories: Vec<Category>,
    pub secret_key: String,
}

#[derive(Debug, Deserialize)]
pub struct GenerateThumbnailRequest {
    #[serde(rename = "videoPath")]
    pub video_path: String,
    #[serde(rename = "timeMs")]
    pub time_ms: i32,
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

#[derive(Debug, Deserialize)]
pub struct FetchMoviePosterRequest {
    #[serde(rename = "videoPath")]
    pub video_path: String,
    #[serde(rename = "movieTitle")]
    pub movie_title: String,
}

#[derive(Debug, Serialize)]
pub struct MoviePosterResult {
    pub title: String,
    pub year: Option<i32>,
    #[serde(rename = "posterPath")]
    pub poster_path: Option<String>,
}

#[derive(Debug, Serialize)]
pub struct ProgressUpdate {
    pub step: String,
    pub progress: i32,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}
