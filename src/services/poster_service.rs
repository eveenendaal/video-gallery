use crate::config::Config;
use crate::models::MoviePosterResult;
use crate::services::thumbnail_service::ProgressCallback;
use anyhow::{anyhow, Result};
use cloud_storage::Client;
use regex::Regex;
use serde::Deserialize;
use std::path::Path;
use std::sync::Arc;
use tokio::fs;
use tokio::io::AsyncWriteExt;
use tracing::{debug, error, info};

#[derive(Debug, Deserialize)]
struct TMDbSearchResponse {
    results: Vec<TMDbMovie>,
}

#[derive(Debug, Deserialize)]
struct TMDbMovie {
    title: String,
    release_date: Option<String>,
    poster_path: Option<String>,
}

#[derive(Clone)]
pub struct PosterService {
    config: Arc<Config>,
    client: reqwest::Client,
}

impl PosterService {
    pub fn new(config: Arc<Config>) -> Self {
        Self {
            config,
            client: reqwest::Client::new(),
        }
    }

    pub async fn search_movie_poster(&self, movie_title: &str) -> Result<Vec<MoviePosterResult>> {
        let api_key = self
            .config
            .tmdb_api_key
            .as_ref()
            .ok_or_else(|| anyhow!("TMDB_API_KEY not configured"))?;

        let url = format!(
            "https://api.themoviedb.org/3/search/movie?api_key={}&query={}",
            api_key,
            urlencoding::encode(movie_title)
        );

        let response = self
            .client
            .get(&url)
            .send()
            .await
            .map_err(|e| anyhow!("Failed to search TMDb: {}", e))?;

        let search_result: TMDbSearchResponse = response
            .json()
            .await
            .map_err(|e| anyhow!("Failed to parse TMDb response: {}", e))?;

        let results = search_result
            .results
            .into_iter()
            .map(|movie| {
                let year = movie
                    .release_date
                    .as_ref()
                    .and_then(|date| date.split('-').next())
                    .and_then(|year_str| year_str.parse::<i32>().ok());

                MoviePosterResult {
                    title: movie.title,
                    year,
                    poster_path: movie.poster_path,
                }
            })
            .collect();

        Ok(results)
    }

    pub async fn fetch_movie_poster(
        &self,
        video_path: &str,
        movie_title: &str,
        progress_cb: Option<ProgressCallback>,
    ) -> Result<()> {
        let send_progress = |step: String, progress: i32| {
            if let Some(cb) = &progress_cb {
                cb(step, progress);
            }
        };

        send_progress("Searching for movie".to_string(), 10);

        // Extract movie title from filename if empty
        let title = if movie_title.is_empty() {
            extract_movie_title(video_path)
        } else {
            movie_title.to_string()
        };

        info!("Searching for movie: {}", title);

        let results = self.search_movie_poster(&title).await?;
        
        if results.is_empty() {
            return Err(anyhow!("No movies found for: {}", title));
        }

        let movie = &results[0];
        let poster_path = movie
            .poster_path
            .as_ref()
            .ok_or_else(|| anyhow!("No poster available for: {}", title))?;

        send_progress("Downloading poster".to_string(), 40);

        let poster_url = format!("https://image.tmdb.org/t/p/w500{}", poster_path);
        let poster_data = self
            .client
            .get(&poster_url)
            .send()
            .await
            .map_err(|e| anyhow!("Failed to download poster: {}", e))?
            .bytes()
            .await
            .map_err(|e| anyhow!("Failed to read poster data: {}", e))?;

        send_progress("Uploading poster to storage".to_string(), 70);

        // Generate thumbnail path
        let thumbnail_path = Self::get_thumbnail_path(video_path);

        let storage_client = Client::default();
        
        // Delete old thumbnail if it exists
        let _ = storage_client
            .object()
            .delete(&self.config.bucket_name, &thumbnail_path)
            .await;

        // Upload poster as thumbnail
        storage_client
            .object()
            .create(
                &self.config.bucket_name,
                poster_data.to_vec(),
                &thumbnail_path,
                "image/jpeg",
            )
            .await
            .map_err(|e| anyhow!("Failed to upload poster: {}", e))?;

        send_progress("Complete".to_string(), 100);
        info!("Successfully fetched poster for {}", title);

        Ok(())
    }

    fn get_thumbnail_path(video_path: &str) -> String {
        let path = Path::new(video_path);
        let stem = path.file_stem().unwrap_or_default();
        let parent = path.parent().unwrap_or_else(|| Path::new(""));
        parent.join(stem).with_extension("jpg").to_string_lossy().to_string()
    }
}

fn extract_movie_title(filename: &str) -> String {
    // Remove path and extension
    let path = Path::new(filename);
    let name = path
        .file_stem()
        .and_then(|s| s.to_str())
        .unwrap_or(filename);

    // Remove year in brackets or parentheses
    let year_regex = Regex::new(r"[\[\(]\d{4}[\]\)]").unwrap();
    let name = year_regex.replace_all(name, "");

    // Replace common separators with spaces
    let name = name.replace(['.', '_', '-'], " ");

    // Remove extra whitespace
    let name = name.split_whitespace().collect::<Vec<_>>().join(" ");

    name.trim().to_string()
}
