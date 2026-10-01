// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useState} from 'react';
import {useIntl} from 'react-intl';

import {Layer} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';

export type LightboxImage = {
    src: string;
    name: string;

    // "2.4 MB", when known.
    size?: string;
    width?: number;
    height?: number;

    // Where to download the original, for uploaded files.
    download?: string;
};

// imageCaption is the mockup's "name · 2.4 MB · 4032×3024".
export function imageCaption(image: LightboxImage): string {
    const parts = [image.name, image.size];
    if (image.width && image.height && image.width > 0 && image.height > 0) {
        parts.push(`${image.width}×${image.height}`);
    }
    return parts.filter(Boolean).join(' · ');
}

// Lightbox shows an image over the whole screen (the mockup's .lightbox), closed by a click or Escape.
function Lightbox({image, onClose}: {image: LightboxImage; onClose: () => void}) {
    const {formatMessage} = useIntl();

    useEffect(() => {
        const onKeyDown = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                e.stopPropagation();
                onClose();
            }
        };
        document.addEventListener('keydown', onKeyDown, true);
        return () => document.removeEventListener('keydown', onKeyDown, true);
    }, [onClose]);

    return (
        <Layer>
            <div
                className={am('lightbox')}
                role='dialog'
                aria-modal='true'
                aria-label={image.name || formatMessage({id: 'fusion.lightbox.label', defaultMessage: 'Image'})}
                onClick={onClose}
            >
                <button
                    type='button'
                    className={am('img-att', 'full')}
                    aria-label={formatMessage({id: 'fusion.lightbox.close', defaultMessage: 'Close image'})}
                    autoFocus={true}
                >
                    <img
                        src={image.src}
                        alt={image.name}
                    />
                    <span>{imageCaption(image)}</span>
                </button>
                {image.download && (
                    <a
                        className={am('btn', 'lightbox-dl')}
                        href={image.download}
                        download={image.name}
                        onClick={(e) => e.stopPropagation()}
                    >
                        {formatMessage({id: 'fusion.lightbox.download', defaultMessage: 'Download'})}
                    </a>
                )}
            </div>
        </Layer>
    );
}

// useLightbox gives a component the lightbox it opens, and the function that opens it.
export function useLightbox(): [React.ReactNode, (image: LightboxImage) => void] {
    const [image, setImage] = useState<LightboxImage | null>(null);
    const close = useCallback(() => setImage(null), []);
    const node = image ? (
        <Lightbox
            image={image}
            onClose={close}
        />
    ) : null;
    return [node, setImage];
}
