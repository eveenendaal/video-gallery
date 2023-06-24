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
  category: string
  stub?: string
  videos?: [Video]
}

type Galleries = Record<string, Gallery>

interface Video {
  name: string
  url: string
  thumbnail?: string
}

async function getGallery (): Promise<Galleries> {
  // Parse the Videos
  const [files] = (await bucket.getFiles())
  const galleries = files.map(file => {
    const parts = file.name.split('/', 3)
    const category = parts[0]
    const group = parts[1]
    const name = parts[2]
    return { category, group, name }
  })
    .filter(file => file.group !== 'thumbnails' && file.name != null)

  return galleries.reduce((accumulator: Galleries,
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
    const stub = current.group = current.group
      .replace(/\s+/g, '-')
      .replace(/[^a-zA-Z0-9-_]/g, '')
      .toLowerCase()

    // Create the gallery if it doesn't exist
    if (accumulator[current.group] == null) {
      accumulator[current.group] = {
        name: current.group,
        category: current.category,
        stub,
        videos: [video]
      }
    } else {
      accumulator[current.group].videos!.push(video)
    }

    console.log(accumulator)

    return accumulator
  }, {})
}

getGallery().then((galleries) => {})

app.get('/', async (req: Request, res: Response): Promise<void> => {
  res.status(200).send()
})

app.get('/feed', async (req: Request, res: Response) => {
  const [files] = (await bucket.getFiles())

  const galleryList: Gallery[] = []
  const prefixes: string[] = []

  // for (const file of files) {
  //   const fileParts = file.name.split('/', 1)
  //   const prefix: string = fileParts[0]
  //   if (prefix != null && !prefixes.includes(prefix)) {
  //     const gallery = galleries[prefix]
  //     prefixes.push(prefix)
  //     galleryList.push({
  //       name: gallery.name,
  //       stub: prefix,
  //       category: gallery.category,
  //       videos: await getVideosByPrefix(prefix)
  //     })
  //   }
  // }

  res.status(200).send(galleryList)
})

function generateSecret (stub: string): string {
  const md5Hasher = crypto.createHmac('md5', 'QuxFzI9lcmwfcg')
  return md5Hasher.update(stub).digest('base64url').slice(0, 4)
}

app.get('/TWs0/index', async (req: Request, res: Response): Promise<void> => {
  const galleryList = new Map()
  // Object.values(Category)
  //   .filter((category, _) => category !== Category.UNKNOWN)
  //   .forEach((category, _) => {
  //     galleryList.set(category.toString(), Object.keys(galleries)
  //       .filter(stub => galleries[stub].category === category)
  //       .map(stub => ({
  //         stub: `/${generateSecret(stub)}/${stub}`,
  //         category: galleries[stub].category,
  //         name: galleries[stub].name
  //       })))
  //   })

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

// app.get('/:password/:gallery', async (req: Request, res: Response): Promise<void> => {
//   const stub = req.params.gallery
//   const gallery = galleries[stub]

//   if (generateSecret(stub) === req.params.password) {
//     res.render('gallery', {
//       gallery: gallery.name,
//       category: gallery.category,
//       videos: await getVideosByPrefix(stub)
//     })
//   } else {
//     res.status(404).send()
//   }
// })

app.set('views', './views')
app.set('view engine', 'pug')

// Start the server
const PORT = process.env.PORT ?? 8080
app.listen(PORT, () => {
  console.log(`App listening on port ${PORT}`)
  console.log('Press Ctrl+C to quit.')
})
