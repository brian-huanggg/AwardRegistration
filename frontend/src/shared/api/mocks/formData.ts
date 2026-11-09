import { File as NodeFile } from 'node:buffer';

/**
 * `request.formData()` for MSW handlers under jsdom. Node's multipart parser builds
 * its parts with `globalThis.File`, which jsdom replaces with a class the parser
 * rejects; Node's own `File` is put back for the duration of the parse.
 */
export async function readFormData(request: Request): Promise<FormData> {
  const jsdomFile = globalThis.File;
  globalThis.File = NodeFile as unknown as typeof File;
  try {
    return await request.formData();
  } finally {
    globalThis.File = jsdomFile;
  }
}
