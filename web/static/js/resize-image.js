// Scale down a user uploaded image for smaller upload sizes.
document.addEventListener('DOMContentLoaded', () => {

    const imageWidth = 1280;
    const imageQuality = 0.9;

    // Compress a JPEG image, returning the blob of a smaller image.
    const compressImage = async (file) => {

        if (!file) return;

        // If the image is not a still picture or is already small enough, return it.
        if (file.type === 'image/gif') {
            console.error("GIF image, return as is");
            return new Promise((resolve) => {
                resolve(file);
            });
        }

        // Get the image bitmap.
        const imageBitmap = await createImageBitmap(file);

        // Already small enough?
        if (imageBitmap.width <= imageWidth && imageBitmap.height <= imageWidth) {
            console.error('small image, return as is');
            return new Promise((resolve) => {
                resolve(file);
            });
        }

        // Compute the height to width ratio.
        let width = imageBitmap.width, height = imageBitmap.height;
        if (imageBitmap.width >= imageBitmap.height) {
            let newWidth = imageWidth
            height = parseInt((height / width) * newWidth);
            width = newWidth;
        } else {
            let newHeight = imageWidth;
            width = parseInt((width / height) * newHeight);
            height = newHeight;
        }

        const canvas = document.createElement('canvas');
        canvas.width = width;
        canvas.height = height;

        const ctx = canvas.getContext('2d');
        ctx.drawImage(imageBitmap, 0, 0, width, height);

        const blob = await new Promise((resolve) => {
            canvas.toBlob(resolve, 'image/jpeg', imageQuality);
        });

        return new File([blob], file.name, {
            type: blob.type,
        });
    };

    // Given an image File, get the compressed image also as a File.
    const compressImageFile = async (file) => {

        if (!file) return;

        // In case the user is on e.g. an old version of Safari <= 14 where the DataTransfer
        // constructor is not available, fail gracefully and return the file untouched.
        let dataTransfer = null;
        try {
            dataTransfer = new DataTransfer();
        } catch(e) {
            console.error("Error compressing the image file: %s", e);
            return {
                fileList: file.files,
                file: file,
            };
        }

        const compressed = await compressImage(file);
        dataTransfer.items.add(compressed);
        return new Promise((resolve) => {
            resolve({
                fileList: dataTransfer.files,
                file: compressed,
            });
        });
    };

    // Export the function.
    window.CompressImageFile = compressImageFile;
});
