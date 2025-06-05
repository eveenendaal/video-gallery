# Video Gallery

The goal of this project is to build a serverless ready application for displaying a users video content library using only a single storage bucket.

## Command Line Interface

Video Gallery now includes a full-featured command-line interface with the following commands:

```
Usage:
  video-gallery [command] [options]

Commands:
  list-categories     List all video categories
  list-galleries      List all galleries
  show-gallery [stub] Show videos in a specific gallery
  export [format]     Export gallery data (formats: json)
  generate-thumbnails Generate thumbnails for videos without existing thumbnails
  serve               Start the web server (original functionality)

Options:
  -h, --help          Show this help message
  -s, --secret-key    Set the SECRET_KEY (overrides environment variable)
  -b, --bucket        Set the BUCKET_NAME (overrides environment variable)
  -p, --port          Set the PORT (overrides environment variable)
```

### Examples

List all categories:
```bash
video-gallery list-categories -s mysecretkey -b mybucket
```

List all galleries:
```bash
video-gallery list-galleries -s mysecretkey -b mybucket
```

Show a specific gallery:
```bash
video-gallery show-gallery gallery-stub-name -s mysecretkey -b mybucket
```

Export gallery data as JSON:
```bash
video-gallery export json -s mysecretkey -b mybucket
```

Generate thumbnails for videos that don't have them:
```bash
video-gallery generate-thumbnails -s mysecretkey -b mybucket
```

Generate thumbnails with additional options:
```bash
video-gallery generate-thumbnails -s mysecretkey -b mybucket -t 3000 -f -o /tmp/thumbs
```
Options:
- `-t, --time` - Time in milliseconds where to extract the thumbnail frame (default: 1000ms)
- `-f, --force` - Force regeneration of thumbnails even if they already exist
- `-o, --output-dir` - Directory for temporary files (default: "thumbnails")

Start the web server:
```bash
video-gallery serve -s mysecretkey -b mybucket -p 8080
```

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

You can navigate to all the galleries from the HTML index page.  After clicking into one of these galleries, the application open a new page specifically for that gallery. Each gallery is given its own unique prefix. This means you'll be able to share an individual gallery with someone without revealing the path to all the galleries.

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
The code obviously could be more organized, but since it only takes a couple hundred lines of code to do what the application needs. I just keep everything in a single file. I'm planning on abtracting the Cloud Run and Cloud Storage Bucket code to allow for the core logic to work with any cloud provider and storage solution.

## Infrastructure
Like I said in the summary, this application can run in Cloud Run for essentially no cost, and only needs a single Storage Bucket to function. Below I will describe the structure of those setups.

### Cloud Run
To get started, simply copy the [docker image](ghcr.io/eveenendaal/video-gallery) to your artifact repository in GCP and start up the image in cloud run. You'll need to configure the application with a service account that has read access to your storage bucket. Next, you'll need to define the following environmental variables.

**BUCKET_NAME** - The bucket with the video files. This is needed to access the bucket.

**SECRET_KEY** - A unique string. This is used to prefix all galleries with a random string to prevent people from guessing the gallery url.

#### Terraform

You can find example terraform code in the [terraform](terraform) directory.

### Running Locally

To run locally, you need to configure the 3 environment variables above as well as set up the default gcp credentials. You can do this by installing the [Google Cloud SDK](https://cloud.google.com/sdk/) and running `gcloud auth login --update-adc`.

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
