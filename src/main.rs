use actix_files as fs;
use actix_web::{web, App, HttpResponse, HttpServer, Responder};
use std::sync::Arc;
use tera::{Context, Tera};

mod config;
mod models;
mod services;

use crate::config::Config;
use crate::services::Service;

async fn gallery_handler(
    service: web::Data<Arc<Service>>,
    tmpl: web::Data<Tera>,
) -> impl Responder {
    let categories = service.get_categories().await;
    let mut context = Context::new();
    context.insert("Categories", &categories);

    // Debug template rendering with detailed error handling
    let rendered = match tmpl.render("index.tera.html", &context) {
        Ok(html) => html,
        Err(err) => {
            eprintln!("Template rendering error (index.tera.html): {:?}", err);
            format!("Template error: {}", err)
        }
    };

    HttpResponse::Ok().content_type("text/html").body(rendered)
}

async fn feed_handler(service: web::Data<Arc<Service>>) -> impl Responder {
    let galleries = service.get_galleries().await;
    HttpResponse::Ok().json(galleries)
}

async fn page_handler(
    service: web::Data<Arc<Service>>,
    tmpl: web::Data<Tera>,
    path: web::Path<String>,
) -> impl Responder {
    let stub = path.into_inner();
    if let Some(gallery) = service.get_gallery(&stub).await {
        let mut context = Context::new();
        context.insert("Name", &gallery.name);
        context.insert("Category", &gallery.category);
        context.insert("Videos", &gallery.videos);

        // Debug template rendering with detailed error handling
        let rendered = match tmpl.render("gallery.tera.html", &context) {
            Ok(html) => html,
            Err(err) => {
                eprintln!("Template rendering error (gallery.tera.html): {:?}", err);
                format!("Template error: {}", err)
            }
        };

        HttpResponse::Ok().content_type("text/html").body(rendered)
    } else {
        HttpResponse::NotFound().body("Gallery not found")
    }
}

#[actix_web::main]
async fn main() -> std::io::Result<()> {
    let config = Arc::new(Config::load().expect("Failed to load config"));
    let port = config.port.clone(); // Extract port before moving config
    let service = Arc::new(Service::new(config.clone()));

    config.print_server_start_message();

    let tera = match Tera::new("templates/**/*") {
        Ok(t) => t,
        Err(e) => {
            println!("Parsing error(s): {}", e);
            ::std::process::exit(1);
        }
    };

    HttpServer::new(move || {
        App::new()
            .app_data(web::Data::new(service.clone()))
            .app_data(web::Data::new(tera.clone()))
            .service(fs::Files::new("/public", "public").show_files_listing())
            .route(
                &format!("/{}/index", config.secret_key),
                web::get().to(gallery_handler),
            )
            .route(
                &format!("/{}/feed", config.secret_key),
                web::get().to(feed_handler),
            )
            .route("/gallery/{stub}", web::get().to(page_handler))
    })
    .bind(format!("0.0.0.0:{}", port))? // Use extracted port
    .run()
    .await
}
