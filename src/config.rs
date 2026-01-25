use std::env;
use thiserror::Error;

#[derive(Debug, Error)]
pub enum ConfigError {
    #[error("SECRET_KEY environment variable not set")]
    SecretKeyNotSet,
    #[error("BUCKET_NAME environment variable not set")]
    BucketNameNotSet,
}

#[derive(Debug, Clone)]
pub struct Config {
    pub secret_key: String,
    pub bucket_name: String,
    pub port: String,
    pub tmdb_api_key: Option<String>,
}

impl Config {
    pub fn load() -> Result<Self, ConfigError> {
        let secret_key = env::var("SECRET_KEY")
            .map_err(|_| ConfigError::SecretKeyNotSet)?;
        
        let bucket_name = env::var("BUCKET_NAME")
            .map_err(|_| ConfigError::BucketNameNotSet)?;
        
        let port = env::var("PORT").unwrap_or_else(|_| "8080".to_string());
        
        let tmdb_api_key = env::var("TMDB_API_KEY").ok();

        Ok(Config {
            secret_key,
            bucket_name,
            port,
            tmdb_api_key,
        })
    }

    pub fn server_address(&self) -> String {
        format!("0.0.0.0:{}", self.port)
    }

    pub fn print_server_start_message(&self) {
        println!("Starting server at port {}", self.port);
        println!("Gallery URL: http://localhost:{}/{}/index", self.port, self.secret_key);
        println!("Feed URL: http://localhost:{}/{}/feed", self.port, self.secret_key);
        println!("Admin URL: http://localhost:{}/{}/admin", self.port, self.secret_key);
    }
}
