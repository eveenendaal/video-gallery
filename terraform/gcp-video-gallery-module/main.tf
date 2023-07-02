// Random String
resource "random_string" "gallery_key" {
  length  = 4
  special = false
}

// Setup Deployment
resource "google_cloud_run_service" "video_gallery" {
  name                       = var.service_name
  location                   = var.default_region
  autogenerate_revision_name = true

  template {
    spec {
      service_account_name = var.service_account_email

      containers {
        image = var.image

        ports {
          container_port = 8080
        }

        env {
          name  = "GCLOUD_PROJECT"
          value = var.project_id
        }

        env {
          name  = "BUCKET_NAME"
          value = var.bucket_name
        }

        env {
          name  = "SECRET_KEY"
          value = random_string.gallery_key.result
        }

        resources {
          limits = {
            cpu    = "1000m"
            memory = "256Mi"
          }
        }
      }
    }

    metadata {
      annotations = {
        "autoscaling.knative.dev/maxScale" = "1"
        "autoscaling.knative.dev/minScale" = "0"
      }
    }
  }

  traffic {
    percent         = 100
    latest_revision = true
  }
}

resource "google_cloud_run_service_iam_policy" "video-gallery-noauth" {
  location    = google_cloud_run_service.video_gallery.location
  project     = google_cloud_run_service.video_gallery.project
  service     = google_cloud_run_service.video_gallery.name
  policy_data = data.google_iam_policy.noauth.policy_data
}

// Adding mappings
resource "google_cloud_run_domain_mapping" "video-gallery" {
  location = var.default_region
  name     = var.domain_name

  metadata {
    namespace = var.project_id
  }

  spec {
    route_name = google_cloud_run_service.video_gallery.name
  }
}

data "google_iam_policy" "noauth" {
  binding {
    role = "roles/run.invoker"
    members = [
      "allUsers",
    ]
  }
}
