use std::path::Path;

/// Generates the thumbnail path from a video path by replacing the extension with .jpg
pub fn get_thumbnail_path(video_path: &str) -> String {
    let path = Path::new(video_path);
    let stem = path.file_stem().unwrap_or_default();
    let parent = path.parent().unwrap_or_else(|| Path::new(""));
    parent
        .join(stem)
        .with_extension("jpg")
        .to_string_lossy()
        .to_string()
}

/// Generates a safe filename by replacing invalid characters
pub fn get_safe_filename(path: &str) -> String {
    path.replace(['/', '\\', ':', '*', '?', '"', '<', '>', '|'], "_")
}
