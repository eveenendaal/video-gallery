use crate::config::Config;
use crate::models::{Category, Gallery, Video};
use anyhow::{anyhow, Result};
use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine;
use cloud_storage::Client;
use moka::future::Cache;
use regex::Regex;
use sha2::{Digest, Sha256};
use std::collections::HashMap;
use std::sync::Arc;
use std::time::Duration;
use tracing::{debug, error, info};

const VIDEO_EXTENSIONS: &[&str] = &[".mp4", ".m4v", ".webm", ".mov", ".avi"];
const IMAGE_EXTENSIONS: &[&str] = &[".jpg", ".jpeg", ".png"];

#[derive(Clone)]
pub struct GalleryService {
    config: Arc<Config>,
    video_cache: Cache<String, Arc<Vec<Video>>>,
}

impl GalleryService {
    pub fn new(config: Arc<Config>) -> Self {
        let video_cache = Cache::builder()
            .time_to_live(Duration::from_secs(300))
            .build();

        Self {
            config,
            video_cache,
        }
    }

    pub async fn get_categories(&self) -> Vec<Category> {
        let galleries = self.get_galleries().await;
        let mut category_map: HashMap<String, Category> = HashMap::new();

        for gallery in galleries {
            let category_name = gallery.category.clone();
            category_map
                .entry(category_name.clone())
                .and_modify(|cat| cat.galleries.push(gallery.clone()))
                .or_insert_with(|| Category {
                    name: category_name.clone(),
                    stub: category_name.clone(),
                    galleries: vec![gallery],
                });
        }

        category_map.into_values().collect()
    }

    pub async fn get_gallery(&self, stub: &str) -> Result<Gallery> {
        let galleries = self.get_galleries().await;
        galleries
            .into_iter()
            .find(|g| g.stub == stub)
            .ok_or_else(|| anyhow!("Gallery not found: {}", stub))
    }

    pub async fn get_galleries(&self) -> Vec<Gallery> {
        let videos = self.get_videos().await;
        let mut gallery_map: HashMap<String, Gallery> = HashMap::new();

        for video in videos {
            let gallery_name = video.gallery.clone();
            
            gallery_map
                .entry(gallery_name.clone())
                .and_modify(|g| g.videos.push(video.clone()))
                .or_insert_with(|| {
                    // Generate hash for gallery URL
                    let mut hasher = Sha256::new();
                    hasher.update(gallery_name.as_bytes());
                    hasher.update(self.config.secret_key.as_bytes());
                    let hash = hasher.finalize();
                    let hash_str = URL_SAFE_NO_PAD.encode(&hash[..3]);

                    Gallery {
                        name: gallery_name.clone(),
                        category: video.category.clone(),
                        stub: format!("/gallery/{}", hash_str),
                        videos: vec![video],
                    }
                });
        }

        let mut galleries: Vec<Gallery> = gallery_map.into_values().collect();
        galleries.sort_by(|a, b| natural_sort(&a.name, &b.name));
        galleries
    }

    pub async fn get_videos(&self) -> Vec<Video> {
        // Check cache first
        if let Some(cached) = self.video_cache.get("videos").await {
            info!("Using cached videos");
            return (*cached).clone();
        }

        info!("Fetching videos from storage");

        let videos = match self.fetch_videos_from_storage().await {
            Ok(videos) => videos,
            Err(e) => {
                error!("Failed to fetch videos: {}", e);
                vec![]
            }
        };

        // Cache the results
        self.video_cache
            .insert("videos".to_string(), Arc::new(videos.clone()))
            .await;

        videos
    }

    async fn fetch_videos_from_storage(&self) -> Result<Vec<Video>> {
        use futures::pin_mut;
        
        let client = Client::default();
        
        // List all objects in the bucket
        let object_stream = client
            .object()
            .list(&self.config.bucket_name, Default::default())
            .await
            .map_err(|e| anyhow!("Failed to list objects: {}", e))?;
        
        pin_mut!(object_stream);

        let extension_regex = Regex::new(r"\.[a-zA-Z0-9]+$")?;
        
        // Collect all object names first
        let mut object_names = Vec::new();
        while let Some(result) = object_stream.next().await {
            let object_list = result.map_err(|e| anyhow!("Error reading object list: {}", e))?;
            
            for object in object_list.items {
                let parts: Vec<&str> = object.name.split('/').collect();
                if parts.len() == 3 && !parts[2].is_empty() {
                    object_names.push(object.name);
                }
            }
        }

        info!("Found {} objects, generating signed URLs in parallel...", object_names.len());

        // Generate signed URLs with limited concurrency to avoid overwhelming the connection pool
        use futures::stream::{self, StreamExt};
        
        let bucket_name = self.config.bucket_name.clone();
        let max_concurrent = 50; // Limit concurrent requests
        
        let signed_urls: Vec<_> = stream::iter(object_names)
            .map(|object_name| {
                let bucket = bucket_name.clone();
                async move {
                    let client = Client::default();
                    match client.object().read(&bucket, &object_name).await {
                        Ok(obj) => match obj.download_url(7 * 24 * 60 * 60) {
                            Ok(url) => Some((object_name, url)),
                            Err(e) => {
                                error!("Failed to generate signed URL for {}: {}", object_name, e);
                                None
                            }
                        },
                        Err(e) => {
                            error!("Failed to read object {}: {}", object_name, e);
                            None
                        }
                    }
                }
            })
            .buffer_unordered(max_concurrent)
            .collect()
            .await;

        // Process results
        let mut videos_map: HashMap<String, Video> = HashMap::new();
        
        for result in signed_urls {
            if let Some((object_name, signed_url)) = result {
                let parts: Vec<&str> = object_name.split('/').collect();
                let category = parts[0];
                let gallery = parts[1];
                let filename = parts[2];

                // Remove extension from filename
                let file_base = extension_regex.replace(filename, "").to_string();

                // Initialize video if it doesn't exist
                if !videos_map.contains_key(&file_base) {
                    videos_map.insert(
                        file_base.clone(),
                        Video {
                            name: file_base.clone(),
                            category: category.to_string(),
                            gallery: gallery.to_string(),
                            url: String::new(),
                            thumbnail: None,
                            video_path: String::new(),
                            thumbnail_path: String::new(),
                        },
                    );
                }

                let video = videos_map.get_mut(&file_base).unwrap();

                // Check if file is a video
                if VIDEO_EXTENSIONS.iter().any(|ext| filename.ends_with(ext)) {
                    video.url = signed_url.clone();
                    video.video_path = object_name.clone();
                }

                // Check if file is a thumbnail
                if IMAGE_EXTENSIONS.iter().any(|ext| filename.ends_with(ext)) {
                    video.thumbnail = Some(signed_url);
                    video.thumbnail_path = object_name.clone();
                }
            }
        }

        let mut videos: Vec<Video> = videos_map.into_values().collect();
        videos.sort_by(|a, b| natural_sort(&a.name, &b.name));

        Ok(videos)
    }

    pub async fn invalidate_cache(&self) {
        self.video_cache.invalidate_all();
        debug!("Video cache invalidated");
    }
}

// Natural sort comparison for strings with numbers
fn natural_sort(s1: &str, s2: &str) -> std::cmp::Ordering {
    use std::cmp::Ordering;
    
    let mut i = 0;
    let mut j = 0;
    let bytes1 = s1.as_bytes();
    let bytes2 = s2.as_bytes();

    while i < bytes1.len() && j < bytes2.len() {
        // Skip spaces
        while i < bytes1.len() && bytes1[i].is_ascii_whitespace() {
            i += 1;
        }
        while j < bytes2.len() && bytes2[j].is_ascii_whitespace() {
            j += 1;
        }

        if i >= bytes1.len() || j >= bytes2.len() {
            break;
        }

        // Check if both are digits
        if bytes1[i].is_ascii_digit() && bytes2[j].is_ascii_digit() {
            let mut num1 = 0u64;
            let mut num2 = 0u64;

            while i < bytes1.len() && bytes1[i].is_ascii_digit() {
                num1 = num1 * 10 + (bytes1[i] - b'0') as u64;
                i += 1;
            }

            while j < bytes2.len() && bytes2[j].is_ascii_digit() {
                num2 = num2 * 10 + (bytes2[j] - b'0') as u64;
                j += 1;
            }

            match num1.cmp(&num2) {
                Ordering::Equal => continue,
                other => return other,
            }
        } else {
            match bytes1[i].cmp(&bytes2[j]) {
                Ordering::Equal => {
                    i += 1;
                    j += 1;
                }
                other => return other,
            }
        }
    }

    bytes1.len().cmp(&bytes2.len())
}

