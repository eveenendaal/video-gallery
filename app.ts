import express, {type Request, type Response} from 'express'
import {Storage} from '@google-cloud/storage'
import * as crypto from 'crypto'
import {File} from "@google-cloud/storage/build/src/file";
import * as path from "path";

const NodeCache = require("node-cache");
const myCache = new NodeCache();

const app = express()

app.disable('x-powered-by')

const projectId = 'veenendaal-base'
const bucketName = 'veenendaal-videos'

const storage = new Storage({projectId})
const bucket = storage.bucket(bucketName)

app.use(express.static('out'))

app.get('/robots.txt', async (req: Request, res: Response): Promise<void> => {
  res.status(200).send('User-agent: *\n' + 'Disallow: /')
})

interface Gallery {
  name: string
  category: string
  stub?: string
  videos?: [Video]
}

interface Video {
  name: string
  url: string
  thumbnail?: string
}

type Galleries = Record<string, Gallery[]>

function generateSecret(stub: string): string {
  const md5Hasher = crypto.createHmac('md5', 'QuxFzI9lcmwfcg')
  return md5Hasher.update(stub).digest('base64url').slice(0, 4)
}

async function getGalleries(): Promise<Gallery[]> {
  // Parse the Videos
  const [files] = (await bucket.getFiles())
  const videoFiles = files.map(file => {
    const parts = file.name.split('/', 3)
    const category = parts[0]
    const group = parts[1]
    const name = parts[2]
    return {category, group, name, filename: file.name}
  })
    .filter(file => file.group !== 'thumbnails' && file.name != null)

  // Sign Urls
  const galleries = [];
  for (const videoFile of videoFiles) {
    let parsedPath = path.parse(videoFile.name);
    const thumbnailFilename = parsedPath.name + ".jpg"

    // Create Stub
    const categoryStub = videoFile.group
      .replace(/\s+/g, '-')
      .replace(/[^a-zA-Z0-9-_]/g, '')
      .toLowerCase()
    const stub = `/${generateSecret(categoryStub)}/${categoryStub}`

    // Create Gallery
    let videoFileName = await signUrl(`${videoFile.filename}`);
    let thumbnailFileName = await signUrl(`${videoFile.category}/${videoFile.group}/thumbnails/${thumbnailFilename}`);

    const gallery = {
      name: videoFile.group,
      category: videoFile.category,
      stub: stub,
      videos: [{
        name: videoFile.name,
        url: videoFileName,
        thumbnail: thumbnailFileName
      } as Video]
    } as Gallery

    galleries.push(gallery);
  }

  return galleries.reduce((accumulator: Gallery[],
                           current: Gallery) => {
    let gallery: Gallery | undefined = accumulator.find(gallery => gallery.name == current.name)
    if (gallery == null) {
      gallery = current
      accumulator.push(gallery)
    } else {
      current.videos?.forEach(video => {
        gallery?.videos?.push(video)
      })
    }
    return accumulator
  }, [])
}

function toDisplay(galleries: Gallery[]): Galleries {
  const result = {} as Galleries
  galleries.forEach(gallery => {
    const stub = gallery.category!
    if (!result[stub]) {
      result[stub] = [gallery]
    } else {
      result[stub].push(gallery)
    }
  })
  return result
}

app.get('/', async (req: Request, res: Response): Promise<void> => {
  res.status(200).send()
})

app.get('/feed', async (req: Request, res: Response) => {
  res.status(200).send(await getGalleries())
})

app.get('/TWs0/index', async (req: Request, res: Response): Promise<void> => {
  res.render('index', {
    galleries: toDisplay(await getGalleries())
  })
})

async function signUrl(filename: string): Promise<string | null> {
  async function signFile(file: File): Promise<string | null> {
    const response = await file.getSignedUrl({
      action: 'read',
      expires: Date.now() + 1000 * 60 * 60 * 24 // one day
    }).catch(error => {
      console.error(error)
    })

    return response?.[0] ?? null
  }

  let cache = await myCache.get(filename);

  if (cache) {
    return cache.result;
  } else {
    const file = await bucket.file(filename);
    const fileExists = (await file.exists())[0]
    const result = fileExists ? (await signFile(file)) : null
    myCache.set(filename, {
      fileExists: fileExists,
      result: result
    }, 600)
    return result
  }
}

app.get('/:password/:gallery', async (req: Request, res: Response): Promise<void> => {
  const stub = `/${req.params.password}/${req.params.gallery}`
  const galleries = (await getGalleries())
  const gallery = galleries.find(gallery => gallery.stub === stub)

  if (gallery) {
    res.render('gallery', gallery)
  } else {
    res.status(404).send()
  }
})

app.set('views', './views')
app.set('view engine', 'pug')

// Start the server
const PORT = process.env.PORT ?? 8080
app.listen(PORT, () => {
  console.log(`App listening on port ${PORT}`)
  console.log('Press Ctrl+C to quit.')
})
