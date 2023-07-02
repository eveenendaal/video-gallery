# Video Gallery

The goal of this project is to build a serverless ready application for displaying a users video content library using only a single storage bucket.

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

**GCLOUD_PROJECT** - The project id you're running the application in. This is needed to build the storage client.

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
    * thumbnail
      * Video.pic (optional)

Here's a real example

* Movies
  * Movies
    * My Movie 1.mp4
    * My Movie 2.mp4
    * thumbnail
      * My Movie 1.jpg
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

