use std::env;

pub struct Config {
    pub secret_key: String,
    pub bucket_name: String,
    pub port: String,
}

impl Config {
    pub fn load() -> Result<Self, String> {
        let secret_key = env::var("SECRET_KEY")
            .map_err(|_| "SECRET_KEY environment variable not set".to_string())?;
        let bucket_name = env::var("BUCKET_NAME")
            .map_err(|_| "BUCKET_NAME environment variable not set".to_string())?;
        let port = env::var("PORT").unwrap_or_else(|_| "8080".to_string());
        Ok(Config {
            secret_key,
            bucket_name,
            port,
        })
    }

    pub fn print_server_start_message(&self) {
        println!("Starting server at port {}", self.port);
        println!("Access the application at: http://localhost:{}", self.port);
        println!(
            "Gallery URL: http://localhost:{}/{}/index",
            self.port, self.secret_key
        );
        println!(
            "Feed URL: http://localhost:{}/{}/feed",
            self.port, self.secret_key
        );
    }
}
