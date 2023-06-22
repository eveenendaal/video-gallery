import express, { type Request, type Response } from 'express'
import { Storage } from '@google-cloud/storage'
import * as crypto from 'crypto'

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
  category: Category
  stub?: string
  videos?: [Video]
}

enum Category {
  VIDEO = 'Videos',
  HOME_VIDEO = 'Home Videos',
  MOVIE = 'Movies',
  UNKNOWN = 'Unknown'
}

type Galleries = Record<string, Gallery>

interface Video {
  name: string
  url: string
  thumbnail?: string
}

// Passwords
const galleries: Galleries = {
  'cindys-tapes': {
    name: "Cindy's Tapes",
    category: Category.HOME_VIDEO
  },
  'dads-tapes': {
    name: "Dad's Tapes",
    category: Category.HOME_VIDEO
  },
  'my-tapes': {
    name: 'My Tapes',
    category: Category.HOME_VIDEO
  },
  'betamax-tapes': {
    name: 'Betamax Tapes',
    category: Category.HOME_VIDEO
  },
  'betamax-original-tapes': {
    name: 'Betamax Tapes (Originals)',
    category: Category.HOME_VIDEO
  },
  'rohrberg-tapes': {
    name: 'Rohrberg Tapes',
    category: Category.HOME_VIDEO
  },
  'mcdaniel-tapes': {
    name: 'McDaniel Tapes',
    category: Category.HOME_VIDEO
  },
  'moms-tapes': {
    name: "Mom's Tapes",
    category: Category.HOME_VIDEO
  },
  '21-day-fix': {
    name: '21 Day Fix',
    category: Category.VIDEO
  },
  'kids-movies': {
    name: "Kid's Movies",
    category: Category.MOVIE
  },
  movies: {
    name: 'Movies',
    category: Category.MOVIE
  }
}

app.get('/', async (req: Request, res: Response): Promise<void> => {
  res.status(200).send()
})

const sampleVideo = 'https://archive.org/download/big-bunny-sample-video/SampleVideo.mp4'
const sampleThumbnail = 'https://eveenendaal.github.io/video-feed-player/example.jpg'

app.get('/demo', async (req: Request, res: Response): Promise<void> => {
  const response = []
  for (let i = 1; i <= 2; i++) {
    const thumbnail = (i % 2) === 0 ? null : sampleThumbnail
    response.push({
      name: `Video Group ${i}`,
      category: `Category ${i}`,
      videos: [{
        name: `Demo Video ${i}`,
        url: sampleVideo,
        thumbnail
      }]
    })
  }
  res.status(200).send(response)
})

app.get('/demo2', async (req: Request, res: Response): Promise<void> => {
  function generateVideos (name: string, category: string): {
    name: string
    stub: string
    videos: any[]
    category: string
  } {
    const videos = []
    for (let i = 1; i <= 10; i++) {
      videos.push({
        name: `${name} Video ${i}`,
        url: sampleVideo
      })
    }

    return {
      name,
      stub: name.toLowerCase().replace(' ', '-'),
      category,
      videos
    }
  }

  const response = []
  response.push(generateVideos('Alice', 'Home Videos'))
  response.push(generateVideos('Bob', 'Home Videos'))
  response.push(generateVideos('Charlie', 'Home Videos'))
  response.push(generateVideos('Movies', 'Movies'))
  response.push(generateVideos('Video', 'Video'))
  res.status(200).send(response)
})

app.get('/feed', async (req: Request, res: Response) => {
  const [files] = (await bucket.getFiles())

  const galleryList: Gallery[] = []
  const prefixes: string[] = []

  for (const file of files) {
    const fileParts = file.name.split('/', 1)
    const prefix: string = fileParts[0]
    if (prefix != null && !prefixes.includes(prefix)) {
      const gallery = galleries[prefix]
      prefixes.push(prefix)
      galleryList.push({
        name: gallery.name,
        stub: prefix,
        category: gallery.category,
        videos: await getVideosByPrefix(prefix)
      })
    }
  }

  res.status(200).send(galleryList)
})

function generateSecret (stub: string): string {
  const md5Hasher = crypto.createHmac('md5', 'QuxFzI9lcmwfcg')
  return md5Hasher.update(stub).digest('base64url').slice(0, 4)
}

app.get('/TWs0/index', async (req: Request, res: Response): Promise<void> => {
  const galleryList = new Map()
  Object.values(Category)
    .filter((category, _) => category !== Category.UNKNOWN)
    .forEach((category, _) => {
      galleryList.set(category.toString(), Object.keys(galleries)
        .filter(stub => galleries[stub].category === category)
        .map(stub => ({
          stub: `/${generateSecret(stub)}/${stub}`,
          category: galleries[stub].category,
          name: galleries[stub].name
        })))
    })

  const displayGalleries: Galleries = {}
  galleryList
    .forEach((value, key) => {
      displayGalleries[key.toString()] = value
    })

  res.render('index', {
    galleries: displayGalleries
  })
})

async function getVideosByPrefix (prefix: string): Promise<[Video]> {
  const [files] = (await bucket.getFiles({ prefix: `${prefix}/`, delimiter: '/' }))
  const thumbnails: Map<string, string> = await getThumbnails(prefix)

  const videos = []
  for (const file of files) {
    const urls: [string] | [] = await file.getSignedUrl({
      action: 'read',
      expires: Date.now() + 1000 * 60 * 60 * 24 // one day
    }).catch(error => { console.error(error) }) ?? []

    const pathParts = file.name.split('/')
    pathParts.splice(0, 1)
    const fileName: string = pathParts.join('/').replace(/\.\w+$/g, '')

    if (urls != null) {
      videos.push({
        name: fileName,
        url: urls[0],
        thumbnail: thumbnails.get(fileName) ?? null
      })
    }
  }

  return videos as [Video]
}

async function getThumbnails (prefix: string): Promise<Map<string, string>> {
  const [files] = (await bucket.getFiles({ prefix: `${prefix}/thumbnails/`, delimiter: '/' }))
  const thumbnails = new Map<string, string>()

  for (const file of files) {
    const urls: [string] | [] = await file.getSignedUrl({
      action: 'read',
      expires: Date.now() + 1000 * 60 * 60 * 24 // one day
    }).catch(error => { console.error(error) }) ?? []

    const pathParts = file.name.split('/')
    pathParts.splice(0, 2)

    const fileName = pathParts.join('/').replace(/\.\w+$/g, '')

    if (urls[0] != null) {
      thumbnails.set(fileName, urls[0])
    }
  }

  return thumbnails
}

app.get('/:password/:gallery', async (req: Request, res: Response): Promise<void> => {
  const stub = req.params.gallery
  const gallery = galleries[stub]

  if (generateSecret(stub) === req.params.password) {
    res.render('gallery', {
      gallery: gallery.name,
      category: gallery.category,
      videos: await getVideosByPrefix(stub)
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
