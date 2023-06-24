import express, {type Request, type Response} from 'express'
import {Storage} from '@google-cloud/storage'
import * as crypto from 'crypto'
import {File} from "@google-cloud/storage/build/src/file";

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

function generateSecret(stub: string): string {
  const md5Hasher = crypto.createHmac('md5', 'QuxFzI9lcmwfcg')
  return md5Hasher.update(stub).digest('base64url').slice(0, 4)
}

async function getGalleries(): Promise<Gallery[]> {
  // Parse the Videos
  const [files] = (await bucket.getFiles())
  const galleries = files.map(file => {
    const parts = file.name.split('/', 3)
    const category = parts[0]
    const group = parts[1]
    const name = parts[2]
    return {category, group, name}
  })
    .filter(file => file.group !== 'thumbnails' && file.name != null)

  return galleries.reduce((accumulator: Gallery[],
                           current: { category: string, group: string, name: string }) => {
    // Get the video url
    const videoUrl = `https://storage.googleapis.com/${bucketName}/${current.category}/${current.group}/${current.name}`
    const thumbnailUrl = `https://storage.googleapis.com/${bucketName}/${current.category}/${current.group}/thumbnails/${current.name}`
    const video: Video = {
      name: current.name,
      url: videoUrl,
      thumbnail: thumbnailUrl
    }

    // Create Stub
    const categoryStub = current.group = current.group
      .replace(/\s+/g, '-')
      .replace(/[^a-zA-Z0-9-_]/g, '')
      .toLowerCase()
    const stub = `${generateSecret(categoryStub)}/${categoryStub}`

    // Create the gallery if it doesn't exist
    let gallery: Gallery | undefined = accumulator.find(gallery => gallery.name == current.group)
    if (gallery == null) {
      gallery = {
        name: current.group,
        category: current.category,
        stub,
        videos: [video]
      }
      accumulator.push(gallery)
    } else {
      gallery.videos!.push(video)
    }
    return accumulator
  }, [])
}

app.get('/', async (req: Request, res: Response): Promise<void> => {
  res.status(200).send()
})

app.get('/feed', async (req: Request, res: Response) => {
  const galleries = await getGalleries()
  res.status(200).send(galleries)
})

app.get('/TWs0/index', async (req: Request, res: Response): Promise<void> => {
  res.render('index', {
    galleries: getGalleries()
  })
})

async function signUrl(prefix: string): Promise<string | null> {
  async function signFile(file: File): Promise<string | null> {
    const response = await file.getSignedUrl({
      action: 'read',
      expires: Date.now() + 1000 * 60 * 60 * 24 // one day
    }).catch(error => {
      console.error(error)
    }) ?? null

    return response?.[0] ?? null
  }

  let response = await bucket.getFiles({prefix: `${prefix}/`, delimiter: '/'});

  const files = response
    .filter(file => file.getSignedUrl !== undefined)
    .map(async (file: File) => await signFile(file))
    .filter(url => url !== null)
    .pop()

  return files!!
}

app.get('/:password/:gallery', async (req: Request, res: Response): Promise<void> => {
  const stub = req.params.gallery
  const galleries = await getGalleries()
  const gallery = galleries.find(gallery => gallery.stub === stub)

  if (generateSecret(stub) === req.params.password) {
    res.render('gallery', {
      gallery
    })
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
