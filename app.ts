import express, {Request, Response} from "express";
import {Storage} from "@google-cloud/storage";

const app = express();

app.disable('x-powered-by');

let projectId = "veenendaal-base";
let bucketName = "veenendaal-videos";

const storage = new Storage({projectId: projectId});
let bucket = storage.bucket(bucketName);

app.use(express.static('out'));

app.get('/robots.txt', async (req, res) => {
    res.status(200).send("User-agent: *\n" + "Disallow: /");
});

interface Gallery {
    name: string
    category: Category
    stub?: string
    videos?: [Video]
    password?: string
}

enum Category {
    VIDEO = "Videos",
    HOME_VIDEO = "Home Videos",
    MOVIE = "Movies",
    UNKNOWN = "Unknown"
}

interface Galleries {
    [key: string]: Gallery;
}

interface Video {
    name: string,
    url: string,
    thumbnail?: string
}

// Passwords
const galleries: Galleries = {
    "cindys-tapes": {
        name: "Cindy's Tapes",
        category: Category.HOME_VIDEO,
        password: "Dh7h"
    },
    "dads-tapes": {
        name: "Dad's Tapes",
        category: Category.HOME_VIDEO,
        password: "1ABF"
    },
    "my-tapes": {
        name: "My Tapes",
        category: Category.HOME_VIDEO,
        password: "drNs"
    },
    "betamax-tapes": {
        name: "Betamax Tapes",
        category: Category.HOME_VIDEO,
        password: "r81q"
    },
    "betamax-original-tapes": {
        name: "Betamax Tapes (Originals)",
        category: Category.HOME_VIDEO,
        password: "0l9I"
    },
    "rohrberg-tapes": {
        name: "Rohrberg Tapes",
        category: Category.HOME_VIDEO,
        password: "cepj"
    },
    "mcdaniel-tapes": {
        name: "McDaniel Tapes",
        category: Category.HOME_VIDEO,
        password: "idkk"
    },
    "moms-tapes": {
        name: "Mom's Tapes",
        category: Category.HOME_VIDEO,
        password: "3b8N"
    },
    "21-day-fix": {
        name: "21 Day Fix",
        category: Category.VIDEO,
        password: "Ihu1"
    },
    "kids-movies": {
        name: "Kid's Movies",
        category: Category.MOVIE,
        password: "Y8DM"
    },
    "movies": {
        name: "Movies",
        category: Category.MOVIE,
        password: "SY7V"
    }
}

app.get('/', async (req: Request, res: Response) => {
    res.status(200).send();
});

app.get("/feed", async (req: Request, res: Response) => {

    const [files] = (await bucket.getFiles());

    const galleryList: Gallery[] = [];
    const prefixes: string[] = []

    for (const file of files) {
        const fileParts = file.name.split("/", 1)
        const prefix: string = fileParts[0]
        if (prefix != null && prefixes.indexOf(prefix) === -1) {
            let gallery = galleries[prefix];
            prefixes.push(prefix)
            galleryList.push({
                name: gallery ? gallery.name : prefix,
                stub: prefix,
                category: gallery ? gallery.category : Category.UNKNOWN,
                videos: await getVideosByPrefix(prefix)
            })
        }
    }

    res.status(200).send(galleryList)
})

app.get('/TWs0/_index', async (req: Request, res: Response) => {

    const galleryList = new Map()
    Object.values(Category)
        .filter((category, _) => category !== Category.UNKNOWN)
        .forEach((category, _) => {
            galleryList.set(category.toString(), Object.keys(galleries)
                .filter(stub => galleries[stub].category === category)
                .map(stub => ({
                    stub: `/${galleries[stub].password}/${stub}`,
                    category: galleries[stub].category,
                    name: galleries[stub].name
                })))
        })

    let jsonObject = {};
    galleryList
        .forEach((value, key) => {
            // @ts-ignore
            jsonObject[key.toString()] = value
        })

    res.render('index', {
        galleries: jsonObject
    });
});

async function getVideosByPrefix(prefix: string): Promise<[Video]> {
    const [files] = (await bucket.getFiles({prefix: `${prefix}/`, delimiter: "/"}));
    const thumbnails: Map<String, String> = await getThumbnails(prefix)

    const videos = [];
    for (const file of files) {
        let urls: [string] | void = await file.getSignedUrl({
            action: 'read',
            expires: Date.now() + 1000 * 60 * 60 * 24, // one day
        }).catch(error => console.error(error));

        const pathParts = file.name.split("/")
        pathParts.splice(0, 1)
        const fileName: string = pathParts.join("/").replace(/\.\w+$/g, "")

        if (urls) {
            videos.push({
                name: fileName,
                url: urls[0],
                thumbnail: thumbnails.get(fileName) ?? null
            });
        }
    }

    return <[Video]>videos
}

async function getThumbnails(prefix: string): Promise<Map<String, String>> {
    const [files] = (await bucket.getFiles({prefix: `${prefix}/thumbnails/`, delimiter: "/"}));
    const thumbnails: Map<string, string> = new Map();

    for (const file of files) {
        let urls: [string] | void = await file.getSignedUrl({
            action: 'read',
            expires: Date.now() + 1000 * 60 * 60 * 24, // one day
        }).catch(error => console.error(error));

        const pathParts = file.name.split("/")
        pathParts.splice(0, 2)

        const fileName = pathParts.join("/").replace(/\.\w+$/g, "")

        if (urls) {
            thumbnails.set(fileName, urls[0])
        }
    }

    return thumbnails
}

app.get('/:password/:gallery', async (req: Request, res: Response) => {
    let stub = req.params.gallery;
    let gallery = galleries[stub];

    if (gallery.password == req.params.password) {
        res.render('gallery', {
            gallery: gallery ? gallery.name : stub,
            category: gallery ? gallery.category : Category.UNKNOWN,
            videos: await getVideosByPrefix(stub)
        });
    } else {
        res.status(404).send()
    }
});

app.set('views', './views');
app.set('view engine', 'pug');

// Start the server
const PORT = process.env.PORT || 8080;
app.listen(PORT, () => {
    console.log(`App listening on port ${PORT}`);
    console.log('Press Ctrl+C to quit.');
});