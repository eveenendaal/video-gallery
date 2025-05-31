use crate::config::Config;
use crate::models::{Category, Gallery, Video};
use cloud_storage::ListRequest;
use cloud_storage::{Client, Object};
use futures_util::{pin_mut, StreamExt};
use regex::Regex;
use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use std::time::{Duration, Instant};

// Simple in-memory cache for videos
struct VideoCache {
    videos: Option<Vec<Video>>,
    expires_at: Option<Instant>,
}

pub struct Service {
    pub config: Arc<Config>,
    cache: Arc<RwLock<VideoCache>>,
}

impl Service {
    pub fn new(config: Arc<Config>) -> Self {
        Service {
            config,
            cache: Arc::new(RwLock::new(VideoCache {
                videos: None,
                expires_at: None,
            })),
        }
    }

    pub async fn get_categories(&self) -> Vec<Category> {
        let galleries = self.get_galleries().await;
        let mut category_map: HashMap<String, Category> = HashMap::new();
        for gallery in galleries {
            let entry = category_map
                .entry(gallery.category.clone())
                .or_insert(Category {
                    name: gallery.category.clone(),
                    stub: gallery.category.clone(),
                    galleries: vec![],
                });
            entry.galleries.push(gallery);
        }
        let mut categories: Vec<Category> = category_map.into_values().collect();
        categories.sort_by(|a, b| natural_cmp(&a.name, &b.name));
        categories
    }

    pub async fn get_galleries(&self) -> Vec<Gallery> {
        let videos = self.get_videos().await;
        let mut gallery_map: HashMap<String, Gallery> = HashMap::new();
        for video in videos {
            let entry = gallery_map.entry(video.gallery.clone()).or_insert_with(|| {
                let hash = sha1_hash(&format!("{}{}", video.gallery, self.config.secret_key));
                Gallery {
                    name: video.gallery.clone(),
                    category: video.category.clone(),
                    stub: format!("/gallery/{}", &hash[..4]),
                    videos: vec![],
                }
            });
            entry.videos.push(video);
        }
        let mut galleries: Vec<Gallery> = gallery_map.into_values().collect();
        galleries.sort_by(|a, b| natural_cmp(&a.name, &b.name));
        galleries
    }

    pub async fn get_gallery(&self, stub: &str) -> Option<Gallery> {
        let galleries = self.get_galleries().await;
        if stub.len() < 4 {
            return None;
        }
        galleries.into_iter().find(|g| g.stub.ends_with(stub))
    }

    pub async fn get_videos(&self) -> Vec<Video> {
        // Simple cache: 5 min expiration
        {
            let cache = self.cache.read().unwrap();
            if let (Some(videos), Some(expires_at)) = (&cache.videos, &cache.expires_at) {
                if *expires_at > Instant::now() {
                    return videos.clone();
                }
            }
        }

        // Google Cloud Storage integration
        let bucket = &self.config.bucket_name;
        let client = Client::default();
        let mut videos_map: HashMap<String, Video> = HashMap::new();

        // Fetch objects using async/await pattern directly
        let mut objects: Vec<Object> = Vec::new();
        let req = ListRequest::default();

        // List objects from bucket
        if let Ok(stream) = client.object().list(bucket, req).await {
            pin_mut!(stream);
            while let Some(result) = stream.next().await {
                if let Ok(page) = result {
                    objects.extend(page.items);
                }
            }
        }

        let video_exts = [".mp4", ".m4v", ".webm", ".mov", ".avi"];
        let image_exts = [".jpg", ".jpeg", ".png"];
        let extension_regex = Regex::new(r"\.[a-zA-Z0-9]+$").unwrap();

        for object in objects {
            let parts: Vec<&str> = object.name.split('/').collect();
            if parts.len() != 3 || parts[2].is_empty() {
                continue;
            }
            let category = parts[0].to_string();
            let gallery = parts[1].to_string();
            let filename = parts[2];
            let file_base = extension_regex.replace(filename, "").to_string();
            let url = format!("https://storage.googleapis.com/{}/{}", bucket, object.name);
            let entry = videos_map.entry(file_base.clone()).or_insert(Video {
                name: file_base.clone(),
                category: category.clone(),
                gallery: gallery.clone(),
                url: String::new(),
                thumbnail: None,
            });
            if video_exts.iter().any(|ext| filename.ends_with(ext)) {
                entry.url = url.clone();
            }
            if image_exts.iter().any(|ext| filename.ends_with(ext)) {
                entry.thumbnail = Some(url.clone());
            }
        }

        let mut videos: Vec<Video> = videos_map.into_values().collect();
        videos.sort_by(|a, b| natural_cmp(&a.name, &b.name));

        // Update cache after collecting data
        let mut cache = self.cache.write().unwrap();
        cache.videos = Some(videos.clone());
        cache.expires_at = Some(Instant::now() + Duration::from_secs(300));

        videos
    }
}

// --- Helpers ---
fn sha1_hash(input: &str) -> String {
    use base64::{engine::general_purpose, Engine as _};
    use sha1::{Digest, Sha1};
    let mut hasher = Sha1::new();
    hasher.update(input.as_bytes());
    let result = hasher.finalize();
    let encoded = general_purpose::URL_SAFE.encode(result);
    encoded
}

fn natural_cmp(a: &str, b: &str) -> std::cmp::Ordering {
    natord::compare(a, b)
}
