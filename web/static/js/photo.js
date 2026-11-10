// Cover photos from the phone camera (Check a book, Paper books).

// shrinkPhoto turns a phone photo into a ~1024 px JPEG data: URL: small and quick to send, and quick for a
// small AI model to read (a title is still easy to read at this size).
export async function shrinkPhoto(file) {
  // "from-image": turn the photo the way the phone was held (older browsers ignore that unless asked).
  const bmp = await createImageBitmap(file, { imageOrientation: "from-image" }).catch(() => createImageBitmap(file));
  const scale = Math.min(1, 1024 / Math.max(bmp.width, bmp.height));
  const canvas = document.createElement("canvas");
  canvas.width = Math.round(bmp.width * scale);
  canvas.height = Math.round(bmp.height * scale);
  canvas.getContext("2d").drawImage(bmp, 0, 0, canvas.width, canvas.height);
  return canvas.toDataURL("image/jpeg", 0.8);
}

// barcodeInPhoto reads an ISBN barcode where the browser can (Chrome on Android).
export async function barcodeInPhoto(file) {
  if (!("BarcodeDetector" in window)) return "";
  try {
    const codes = await new window.BarcodeDetector({ formats: ["ean_13"] }).detect(await createImageBitmap(file));
    return codes.map((c) => c.rawValue).find((v) => /^97[89]\d{10}$/.test(v)) || "";
  } catch {
    return "";
  }
}
