use crate::config::Config;
use anyhow::{anyhow, Result};
use cloud_storage::Client;
use std::path::{Path, PathBuf};
use std::sync::Arc;
use tokio::fs;
use tokio::io::AsyncWriteExt;
use tokio::process::Command;
use tracing::{debug, error, info};

pub type ProgressCallback = Arc<dyn Fn(String, i32) + Send + Sync>;

#[derive(Clone)]
pub struct ThumbnailService {
    config: Arc<Config>,
}

impl ThumbnailService {
    pub fn new(config: Arc<Config>) -> Self {
        Self { config }
    }

    pub async fn generate_thumbnail(&self, video_path: &str, time_ms: i32) -> Result<()> {
        self.generate_thumbnail_with_progress(video_path, time_ms, None)
            .await
    }

    pub async fn generate_thumbnail_with_progress(
        &self,
        video_path: &str,
        time_ms: i32,
        progress_cb: Option<ProgressCallback>,
    ) -> Result<()> {
        let send_progress = |step: String, progress: i32| {
            if let Some(cb) = &progress_cb {
                cb(step, progress);
            }
        };

        send_progress("Checking FFmpeg".to_string(), 5);
        check_ffmpeg().await?;

        send_progress("Setting up directories".to_string(), 10);
        let output_dir = std::env::temp_dir().join("video-gallery-thumbnails");
        fs::create_dir_all(&output_dir).await?;

        send_progress("Connecting to storage".to_string(), 15);
        let client = Client::default();

        // Generate thumbnail path
        let thumbnail_path = Self::get_thumbnail_path(video_path);

        // Generate safe filenames
        let video_basename = get_safe_filename(video_path);
        let thumbnail_basename = get_safe_filename(&thumbnail_path);

        send_progress("Clearing old thumbnail".to_string(), 20);
        // Delete old thumbnail if it exists (ignore errors)
        let _ = client
            .object()
            .delete(&self.config.bucket_name, &thumbnail_path)
            .await;

        send_progress("Downloading video".to_string(), 30);
        let tmp_video_path = output_dir.join(&video_basename);
        self.download_file(&client, video_path, &tmp_video_path)
            .await?;

        // Ensure cleanup
        let _guard = FileGuard::new(tmp_video_path.clone());

        send_progress("Generating thumbnail".to_string(), 60);
        let tmp_thumbnail_path = output_dir.join(&thumbnail_basename);
        create_thumbnail_with_ffmpeg(&tmp_video_path, &tmp_thumbnail_path, time_ms).await?;

        let _thumb_guard = FileGuard::new(tmp_thumbnail_path.clone());

        send_progress("Uploading thumbnail".to_string(), 80);
        self.upload_file(&client, &tmp_thumbnail_path, &thumbnail_path)
            .await?;

        send_progress("Complete".to_string(), 100);
        info!("Successfully generated thumbnail for {}", video_path);

        Ok(())
    }

    pub async fn clear_thumbnail(&self, thumbnail_path: &str) -> Result<()> {
        let client = Client::default();
        
        client
            .object()
            .delete(&self.config.bucket_name, thumbnail_path)
            .await
            .map_err(|e| anyhow!("Failed to delete thumbnail: {}", e))?;

        info!("Cleared thumbnail: {}", thumbnail_path);
        Ok(())
    }

    pub async fn bulk_generate_thumbnails(
        &self,
        video_paths: Vec<String>,
        time_ms: i32,
        max_parallel: usize,
    ) -> (usize, usize) {
        use futures::stream::{self, StreamExt};

        let results: Vec<Result<()>> = stream::iter(video_paths)
            .map(|path| {
                let service = self.clone();
                async move { service.generate_thumbnail(&path, time_ms).await }
            })
            .buffer_unordered(max_parallel)
            .collect()
            .await;

        let success = results.iter().filter(|r| r.is_ok()).count();
        let failed = results.len() - success;

        (success, failed)
    }

    pub async fn bulk_clear_thumbnails(&self, thumbnail_paths: Vec<String>) -> usize {
        let mut cleared = 0;
        for path in thumbnail_paths {
            if self.clear_thumbnail(&path).await.is_ok() {
                cleared += 1;
            }
        }
        cleared
    }

    fn get_thumbnail_path(video_path: &str) -> String {
        let path = Path::new(video_path);
        let stem = path.file_stem().unwrap_or_default();
        let parent = path.parent().unwrap_or_else(|| Path::new(""));
        parent.join(stem).with_extension("jpg").to_string_lossy().to_string()
    }

    async fn download_file(&self, client: &Client, object_path: &str, dest_path: &Path) -> Result<()> {
        let data = client
            .object()
            .download(&self.config.bucket_name, object_path)
            .await
            .map_err(|e| anyhow!("Failed to download {}: {}", object_path, e))?;

        let mut file = fs::File::create(dest_path).await?;
        file.write_all(&data).await?;

        debug!("Downloaded {} to {:?}", object_path, dest_path);
        Ok(())
    }

    async fn upload_file(&self, client: &Client, src_path: &Path, object_path: &str) -> Result<()> {
        let data = fs::read(src_path).await?;

        client
            .object()
            .create(
                &self.config.bucket_name,
                data,
                object_path,
                "image/jpeg",
            )
            .await
            .map_err(|e| anyhow!("Failed to upload {}: {}", object_path, e))?;

        debug!("Uploaded {:?} to {}", src_path, object_path);
        Ok(())
    }
}

async fn check_ffmpeg() -> Result<()> {
    Command::new("ffmpeg")
        .arg("-version")
        .output()
        .await
        .map_err(|_| anyhow!("FFmpeg is required but not found"))?;
    Ok(())
}

async fn create_thumbnail_with_ffmpeg(
    video_path: &Path,
    output_path: &Path,
    time_ms: i32,
) -> Result<()> {
    let time_sec = time_ms as f64 / 1000.0;

    let output = Command::new("ffmpeg")
        .arg("-ss")
        .arg(format!("{}", time_sec))
        .arg("-i")
        .arg(video_path)
        .arg("-vframes")
        .arg("1")
        .arg("-q:v")
        .arg("2")
        .arg("-y")
        .arg(output_path)
        .output()
        .await?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        error!("FFmpeg error: {}", stderr);
        return Err(anyhow!("FFmpeg failed to generate thumbnail"));
    }

    debug!("Created thumbnail at {:?}", output_path);
    Ok(())
}

fn get_safe_filename(path: &str) -> String {
    path.replace(['/', '\\', ':', '*', '?', '"', '<', '>', '|'], "_")
}

// RAII guard for file cleanup
struct FileGuard {
    path: PathBuf,
}

impl FileGuard {
    fn new(path: PathBuf) -> Self {
        Self { path }
    }
}

impl Drop for FileGuard {
    fn drop(&mut self) {
        if self.path.exists() {
            let _ = std::fs::remove_file(&self.path);
        }
    }
}
