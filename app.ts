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
    get title(): string
}

interface Galleries {
    [key: string]: Gallery;
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
    }
}

app.get('/:gallery', async (req: Request, res: Response) => {
    let gallery = req.params.gallery;

    const [response] = (await bucket.getFiles({directory: gallery, delimiter: "/"}));

    const videos = [];
    for (const file of response) {
        let urls: [string] | void = await file.getSignedUrl({
            action: 'read',
            expires: Date.now() + 1000 * 60 * 60, // one hour
        }).catch(error => console.error(error));

        const pathParts = file.name.split("/")
        pathParts.splice(0, 1)

        videos.push({
            name: pathParts.join("/").replace(/\.\w+$/g, ""),
            url: urls ? urls[0] : null
        });
    }

    res.render('index', {
        gallery: galleries[gallery] ? galleries[gallery].title : gallery,
        videos: videos
    });
});

app.get('/', async (req: Request, res: Response) => {
    res.status(200).send();
});

app.set('views', './views');
app.set('view engine', 'pug');

// Start the server
const PORT = process.env.PORT || 8080;
app.listen(PORT, () => {
    console.log(`App listening on port ${PORT}`);
    console.log('Press Ctrl+C to quit.');
});