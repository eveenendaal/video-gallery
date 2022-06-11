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
    title: string
    videos?: [Video]
}

interface Galleries {
    [key: string]: Gallery;
}

interface Video {
    name: string,
    url: string
}

const galleries: Galleries = {
    "cindys-tapes": {
        title: "Cindy's Tapes"
    },
    "dads-tapes": {
        title: "Dad's Tapes"
    },
    "my-tapes": {
        title: "My Tapes"
    },
    "betamax-tapes": {
        title: "Betamax Tapes"
    },
    "betamax-original-tapes": {
        title: "Betamax Tapes (Originals)"
    },
    "rohrberg-tapes": {
        title: "Rohrberg Tapes"
    },
    "mcdaniel-tapes": {
        title: "McDaniel Tapes"
    },
    "moms-tapes": {
        title: "Mom's Tapes"
    },
    "21-day-fix": {
        title: "21 Day Fix"
    },
    "kids-movies": {
        title: "Kid's Movies"
    },
    "movies": {
        title: "Movies"
    }
}

app.get('/', async (req: Request, res: Response) => {
    res.status(200).send();
});

app.get("/feed", async (req: Request, res: Response) => {

    const [files] = (await bucket.getFiles());

    const videos: Galleries = {};

    for (const file of files) {
        const fileParts = file.name.split("/", 1)
        const prefix: string = fileParts[0]
        if (!videos[prefix]) {
            videos[prefix] = {
                title: galleries[prefix] ? galleries[prefix].title : prefix,
                videos: await getVideosByPrefix(prefix)
            }
        }
    }

    res.status(200).send(videos)
})

app.get('/_index', async (req: Request, res: Response) => {
    const index = Object.keys(galleries)
        .map(stub => ({
            stub: `/${stub}`,
            name: galleries[stub].title
        }))

    res.render('index', {
        galleries: index
    });
});

async function getVideosByPrefix(prefix: string): Promise<[Video]> {
    const [files] = (await bucket.getFiles({prefix: `${prefix}/`, delimiter: "/"}));

    const videos = [];
    for (const file of files) {
        let urls: [string] | void = await file.getSignedUrl({
            action: 'read',
            expires: Date.now() + 1000 * 60 * 60 * 24, // one day
        }).catch(error => console.error(error));

        const pathParts = file.name.split("/")
        pathParts.splice(0, 1)

        videos.push({
            name: pathParts.join("/").replace(/\.\w+$/g, ""),
            url: urls ? urls[0] : null
        });
    }

    return <[Video]>videos
}

app.get('/:gallery', async (req: Request, res: Response) => {
    let gallery = req.params.gallery;

    res.render('gallery', {
        gallery: galleries[gallery] ? galleries[gallery].title : gallery,
        videos: await getVideosByPrefix(gallery)
    });
});

app.set('views', './views');
app.set('view engine', 'pug');

// Start the server
const PORT = process.env.PORT || 8080;
app.listen(PORT, () => {
    console.log(`App listening on port ${PORT}`);
    console.log('Press Ctrl+C to quit.');
});