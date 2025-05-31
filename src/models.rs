use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct Category {
    pub name: String,
    pub stub: String,
    pub galleries: Vec<Gallery>,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct Gallery {
    pub name: String,
    pub category: String,
    pub stub: String,
    pub videos: Vec<Video>,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct Video {
    pub name: String,
    #[serde(skip_serializing)]
    pub category: String,
    #[serde(skip_serializing)]
    pub gallery: String,
    pub url: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub thumbnail: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Index {
    pub categories: Vec<Category>,
}
