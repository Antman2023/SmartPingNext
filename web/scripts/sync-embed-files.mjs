import fs from 'node:fs'
import { join, resolve } from 'node:path'

export function syncEmbeddedFiles(source, staticDirectory) {
  const staticRoot = resolve(staticDirectory)
  const destination = join(staticRoot, 'html')
  if (!fs.statSync(join(source, 'index.html')).isFile()) {
    throw new Error('Build the frontend before synchronizing embedded files.')
  }
  // An ancestor may legitimately be a symlink (for example, /var points to
  // /private/var on macOS). Only reject a redirect at the directory whose
  // contents this function replaces.
  if (fs.lstatSync(staticRoot).isSymbolicLink()) {
    throw new Error('The static directory must not redirect outside the project.')
  }

  // All rename and cleanup targets remain within the verified static directory.
  const temporary = fs.mkdtempSync(join(staticRoot, '.html-sync-'))
  const staged = join(temporary, 'next')
  const backup = join(temporary, 'previous')
  let preserveBackup = false
  try {
    fs.cpSync(source, staged, { recursive: true })
    let hadPrevious = false
    try {
      fs.renameSync(destination, backup)
      hadPrevious = true
    } catch (error) {
      if (error.code !== 'ENOENT') throw error
    }
    try {
      fs.renameSync(staged, destination)
    } catch (error) {
      if (hadPrevious) {
        try {
          fs.renameSync(backup, destination)
        } catch (restoreError) {
          preserveBackup = true
          throw new AggregateError(
            [error, restoreError],
            `Cannot restore embedded files. Previous build retained at ${backup}`,
            { cause: restoreError }
          )
        }
      }
      throw error
    }
  } finally {
    if (!preserveBackup) fs.rmSync(temporary, { recursive: true, force: true })
  }
}
