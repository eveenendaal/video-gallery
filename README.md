# Video Gallery

The goal of this project is to build a serverless ready application for displaying a users video content library using only a single storage bucket.

## Overview

Video Gallery is a web-based application that runs as a Docker container, providing an interface to browse and play videos organized in galleries. The application is designed to run on serverless platforms like Google Cloud Run, requiring only a single storage bucket for video files.

## Web Interface

The interface is pretty simple.

For the HTML index use:
```
GET /{SECRET_KEY}/index
```

For the Video Feed use:
```
GET /{SECRET_KEY}/feed
```

For the Admin interface use:
```
GET /{SECRET_KEY}/admin
```

**Security Note:** All admin endpoints (page and API) are protected by the secret key in the URL path. The admin API endpoints follow the pattern `/{SECRET_KEY}/admin/api/*`, ensuring that only users with knowledge of the secret key can perform administrative operations.

You can navigate to all the galleries from the HTML index page.  After clicking into one of these galleries, the application open a new page specifically for that gallery. Each gallery is given its own unique prefix. This means you'll be able to share an individual gallery with someone without revealing the path to all the galleries.

### Admin Interface

The admin interface provides powerful tools for managing video thumbnails:

**Features:**
- View all videos with their current thumbnail status
- Generate thumbnails from video frames with customizable time offset (for home movies)
- Fetch movie posters from TMDb database (for actual movies)
- Clear thumbnails for individual videos
- Filter and sort videos by category, gallery, or status
- Select multiple videos for bulk operations

**Individual Video Operations:**
Each video has controls to:
1. Choose between "From Video" (extract frame) or "Movie Poster" (fetch from TMDb)
2. For "From Video": Set the time in milliseconds where the thumbnail should be extracted (e.g., 1000ms = 1 second into the video)
3. For "Movie Poster": The system automatically extracts the movie title from the filename and searches TMDb
4. Generate/fetch the thumbnail
5. Clear the existing thumbnail (if present)

**Bulk Operations:**
1. Select multiple videos using checkboxes
2. Set a default time offset for frame extraction
3. Click "Generate Selected" to create thumbnails for selected videos
4. Click "Clear Selected" to remove thumbnails from selected videos
5. Adjust parallel operations (1-10) to control processing speed

**Movie Poster Feature:**
When using "Movie Poster" mode, the system:
- Automatically extracts the movie title from the video filename
- Removes file extensions, years in brackets, and normalizes formatting
- Searches The Movie Database (TMDb) for matching movies
- Downloads and uploads the official movie poster as the thumbnail
- Requires TMDB_API_KEY environment variable to be set

The interface will show real-time progress of each operation and automatically refresh after successful completion.

## Feed Schema

Below is the formal schema for the video feed the player expects.

```
{
    "$schema": "http://json-schema.org/draft-04/schema#",
    "type": "array",
    "items": [
        {
            "type": "object",
            "properties": {
                "name": {
                    "type": "string"
                },
                "category": {
                    "type": "string"
                },
                "videos": {
                    "type": "array",
                    "items": [
                        {
                            "type": "object",
                            "properties": {
                                "name": {
                                    "type": "string"
                                },
                                "url": {
                                    "type": "string"
                                },
                                "thumbnail": {
                                    "type": "string"
                                }
                            },
                            "required": [
                                "name",
                                "url"
                            ]
                        }
                    ]
                }
            },
            "required": [
                "name",
                "category",
                "videos"
            ]
        }
    ]
}
```

### Feed Example

This is an example video feed. The URL and thumbnail values are just placeholders.

```
[
    {
        "name": "Video Group 1",
        "category": "Category 1",
        "videos": [
            {
                "name": "Demo Video 1",
                "url": "https://domain.tld/video-1.mp4",
                "thumbnail": "https://domain.tld/example.jpg"
            }
        ]
    },
    {
        "name": "Video Group 2",
        "category": "Category 2",
        "videos": [
            {
                "name": "Demo Video 2",
                "url": "https://domain.tld/video-2.mp4",
                "thumbnail": null
            }
        ]
    }
]
```

### Integrations

#### [Video Feed Player](https://www.ericveenendaal.com/blog/video-feed-player)
This tvOS application is compatible with this video feed

## Code Structure

The project follows a standard Go application structure:

```
.
├── api/              # API documentation and test requests
├── assets/           # Frontend assets
│   ├── scss/        # SASS stylesheets
│   └── templates/   # Pug templates
├── build/           # Build and deployment files (Dockerfile, etc.)
├── cmd/             # CLI command implementations
├── config/          # Configuration files
├── docs/            # Project documentation
├── pkg/             # Go packages
│   ├── config/      # Configuration management
│   ├── handlers/    # HTTP request handlers
│   ├── models/      # Data models
│   └── services/    # Business logic services
├── public/          # Static web assets
├── schemas/         # JSON schemas
├── scripts/         # Build and automation scripts
└── terraform/       # Infrastructure as code
```

## Infrastructure
Like I said in the summary, this application can run in Cloud Run for essentially no cost, and only needs a single Storage Bucket to function. Below I will describe the structure of those setups.

### Cloud Run

The application is available as a Docker image at `ghcr.io/eveenendaal/video-gallery:latest`. 

To deploy to Cloud Run:
1. Pull the Docker image from GitHub Container Registry or use it directly in Cloud Run
2. Configure a service account with read access to your storage bucket
3. Set the following environment variables:

**BUCKET_NAME** - The bucket with the video files. This is needed to access the bucket.

**SECRET_KEY** - A unique string. This is used to prefix all galleries with a random string to prevent people from guessing the gallery url.

**TMDB_API_KEY** (Optional) - API key from The Movie Database (TMDb) for fetching movie posters. Required if you want to use the "Movie Poster" feature in the admin panel. Get a free API key at https://www.themoviedb.org/settings/api

#### Terraform

You can find example terraform code in the [terraform](terraform) directory.

### Running Locally

To run the application locally using Docker:

```bash
docker run -p 8080:8080 \
  -e SECRET_KEY=your-secret-key \
  -e BUCKET_NAME=your-bucket-name \
  -e TMDB_API_KEY=your-tmdb-key \
  -v ~/.config/gcloud:/home/appuser/.config/gcloud:ro \
  ghcr.io/eveenendaal/video-gallery:latest
```

You need to configure the environment variables above as well as set up the default GCP credentials. You can do this by installing the [Google Cloud SDK](https://cloud.google.com/sdk/) and running `gcloud auth login --update-adc`, then mounting the credentials directory into the container as shown above.

### Storage Bucket
The application assumes the Storage Bucket is stored as follows:

* Category
  * Group
    * Video.vid
    * Video.pic (optional)

Here's a real example

* Movies
  * Movies
    * My Movie 1.mp4
    * My Movie 1.jpg
    * My Movie 2.mp4
    * My Movie 2.jpg
* Home Videos
  * Bob
    * Video of Bob 1.mp4
    * Video of Bob 2.mp4
  * Alice
    * Video of Alice 1.mp4
    * Video of Alice 2.mp4
    * Video of Alice 3.mp4

The code parses the bucket and creates a list of categories, groups, and videos. The code also looks for a thumbnail for each video. If a thumbnail is not found, the thumbnail url will be null.
