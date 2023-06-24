import express, { type Request, type Response } from 'express'
import { Storage } from '@google-cloud/storage'
import * as crypto from 'crypto'
import { type File } from '@google-cloud/storage/build/src/file'
import * as path from 'path'
import NodeCache from 'node-cache'

const myCache = new NodeCache()

const app = express()

app.disable('x-powered-by')

const projectId = 'veenendaal-base'
const bucketName = 'veenendaal-videos'

const storage = new Storage({ projectId })
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
  thumbnail: string | null
}

type Galleries = Record<string, Gallery[]>
interface CacheResult {
  fileExists: boolean
  result: string | null
}

function generateSecret (stub: string): string {
  const md5Hasher = crypto.createHmac('md5', 'QuxFzI9lcmwfcg')
  return md5Hasher.update(stub).digest('base64url').slice(0, 4)
}

async function getGalleries (): Promise<Gallery[]> {
  // Parse the Videos
  const [files] = (await bucket.getFiles())
  const videoFiles = files.map(file => {
    const parts = file.name.split('/', 3)
    const category = parts[0]
    const group = parts[1]
    const name = parts[2]
    return { category, group, name, filename: file.name }
  })
    .filter(file => file.group !== 'thumbnails' && file.name != null)

  // Sign Urls
  const galleries = []
  for (const videoFile of videoFiles) {
    const parsedPath = path.parse(videoFile.name)
    const thumbnailFilename = parsedPath.name + '.jpg'

    // Create Stub
    const categoryStub = videoFile.group
      .replace(/\s+/g, '-')
      .replace(/[^a-zA-Z0-9-_]/g, '')
      .toLowerCase()
    const stub = `/${generateSecret(categoryStub)}/${categoryStub}`

    // Create Gallery
    const videoFileName = await signUrl(`${videoFile.filename}`)
    const thumbnailFileName = await signUrl(`${videoFile.category}/${videoFile.group}/thumbnails/${thumbnailFilename}`)

    const gallery: Gallery = {
      name: videoFile.group,
      category: videoFile.category,
      stub,
      videos: [{
        name: videoFile.name,
        url: videoFileName ?? '',
        thumbnail: thumbnailFileName
      }]
    }

    galleries.push(gallery)
  }

  return galleries.reduce((accumulator: Gallery[],
    current: Gallery) => {
    let gallery: Gallery | undefined = accumulator.find(gallery => gallery.name === current.name)
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

function toDisplay (galleries: Gallery[]): Galleries {
  const result: Galleries = {}
  galleries.forEach(gallery => {
    const stub = gallery.category
    if (result[stub] == null) {
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

async function signUrl (filename: string): Promise<string | null> {
  async function signFile (file: File): Promise<string | null> {
    const response = await file.getSignedUrl({
      action: 'read',
      expires: Date.now() + 1000 * 60 * 60 * 24 // one day
    }).catch(error => {
      console.error(error)
    })

    return response?.[0] ?? null
  }

  const cache = await myCache.get(filename) as CacheResult

  if (cache != null) {
    return cache.result
  } else {
    const file = bucket.file(filename)
    const fileExists = (await file.exists())[0]
    const result = fileExists ? (await signFile(file)) : null
    myCache.set(filename, {
      fileExists,
      result
    }, 600)
    return result
  }
}

app.get('/:password/:gallery', async (req: Request, res: Response): Promise<void> => {
  const stub = `/${req.params.password}/${req.params.gallery}`
  const galleries = (await getGalleries())
  const gallery = galleries.find(gallery => gallery.stub === stub)

  if (gallery != null) {
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
