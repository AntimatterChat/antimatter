// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useState} from 'react';
import {useIntl} from 'react-intl';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import type {IconName} from 'fusion/components/icon';
import {Layer} from 'fusion/components/layer';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {useWhen} from 'fusion/messages/time';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';
import {copyToClipboard} from 'utils/utils';

export type LightboxImage = {
    src: string;
    name: string;

    // "2.4 MB", when known.
    size?: string;
    width?: number;
    height?: number;

    // Where to download the original, for uploaded files.
    download?: string;

    // A link to it to share, and the classic file preview, for uploaded files.
    link?: string;
    onPreview?: () => void;

    // Who posted it and when, shown at the top as Discord's viewer does.
    userId?: string;
    at?: number;
};

// LightboxAuthor is who posted the image, and when, at the top left of the viewer.
function LightboxAuthor({userId, at}: {userId: string; at?: number}) {
    const user = useUser(userId);
    const name = useDisplayName(user);
    const when = useWhen();
    return (
        <span className={am('lb-author')}>
            <Avatar
                userId={userId}
                size='sm'
            />
            <span>
                <b>{name}</b>
                {at ? <small>{when(at)}</small> : null}
            </span>
        </span>
    );
}

// imageCaption is the mockup's "name · 2.4 MB · 4032×3024".
export function imageCaption(image: LightboxImage): string {
    const parts = [image.name, image.size];
    if (image.width && image.height && image.width > 0 && image.height > 0) {
        parts.push(`${image.width}×${image.height}`);
    }
    return parts.filter(Boolean).join(' · ');
}

// Lightbox shows an image over the whole screen (the mockup's .lightbox): the other images of the message with the
// arrows (or ← →), zoom with the buttons, the wheel or a double click (+ and − too), and the image's actions (copy
// its link, download, the classic preview with more). A click outside the image or Escape closes it.
function Lightbox({images, start, onClose}: {images: LightboxImage[]; start: number; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const toast = useToast();
    const [index, setIndex] = useState(start);
    const [zoom, setZoom] = useState(1);
    const image = images[Math.min(index, images.length - 1)];
    const many = images.length > 1;
    const go = useCallback((step: number) => {
        setIndex((i) => (i + step + images.length) % images.length);
        setZoom(1);
    }, [images.length]);
    const zoomBy = useCallback((factor: number) => setZoom((z) => Math.min(8, Math.max(1, Math.round(z * factor * 100) / 100))), []);

    useEffect(() => {
        const onKeyDown = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                e.stopPropagation();
                onClose();
            } else if (e.key === 'ArrowRight' && many) {
                go(1);
            } else if (e.key === 'ArrowLeft' && many) {
                go(-1);
            } else if (e.key === '+' || e.key === '=') {
                zoomBy(1.5);
            } else if (e.key === '-') {
                zoomBy(1 / 1.5);
            } else if (e.key === '0') {
                setZoom(1);
            }
        };
        document.addEventListener('keydown', onKeyDown, true);
        return () => document.removeEventListener('keydown', onKeyDown, true);
    }, [onClose, go, zoomBy, many]);

    const tool = (label: string, icon: IconName | 'zoom-in' | 'zoom-out', onClick: () => void, disabled = false) => (
        <button
            type='button'
            className={am('lb-tool')}
            title={label}
            aria-label={label}
            disabled={disabled}
            onClick={(e) => {
                e.stopPropagation();
                onClick();
            }}
        >
            {icon === 'zoom-in' || icon === 'zoom-out' ? <b aria-hidden='true'>{icon === 'zoom-in' ? '+' : '−'}</b> : (
                <Icon
                    name={icon}
                    size='sm'
                />
            )}
        </button>
    );

    return (
        <Layer>
            <div
                className={am('lightbox', {zoomed: zoom > 1})}
                role='dialog'
                aria-modal='true'
                aria-label={image.name || formatMessage({id: 'fusion.lightbox.label', defaultMessage: 'Image'})}
                onClick={onClose}
                onWheel={(e) => {
                    if (e.ctrlKey || zoom > 1 || e.deltaY < 0) {
                        zoomBy(e.deltaY < 0 ? 1.15 : 1 / 1.15);
                    }
                }}
            >
                <div
                    className={am('lb-stage')}
                    onClick={(e) => e.target === e.currentTarget && onClose()}
                >
                    <img
                        key={image.src}
                        src={image.src}
                        alt={image.name}
                        style={zoom > 1 ? {width: `${zoom * 100}%`, maxWidth: 'none', maxHeight: 'none'} : undefined}
                        onClick={(e) => e.stopPropagation()}
                        onDoubleClick={() => setZoom(zoom > 1 ? 1 : 2.5)}
                    />
                </div>
                <div
                    className={am('lb-top')}
                    onClick={(e) => e.stopPropagation()}
                >
                    {image.userId ? (
                        <LightboxAuthor
                            userId={image.userId}
                            at={image.at}
                        />
                    ) : <span className={am('grow')}/>}
                    {tool(formatMessage({id: 'fusion.lightbox.zoomOut', defaultMessage: 'Zoom out (−)'}), 'zoom-out', () => zoomBy(1 / 1.5), zoom <= 1)}
                    <span className={am('lb-zoom')}>{`${Math.round(zoom * 100)}%`}</span>
                    {tool(formatMessage({id: 'fusion.lightbox.zoomIn', defaultMessage: 'Zoom in (+)'}), 'zoom-in', () => zoomBy(1.5), zoom >= 8)}
                    {image.link && tool(formatMessage({id: 'fusion.lightbox.copyLink', defaultMessage: 'Copy link'}), 'link', () => {
                        copyToClipboard(image.link!);
                        toast(formatMessage({id: 'fusion.toast.linkCopied', defaultMessage: 'Link copied'}));
                    })}
                    {image.download && (
                        <a
                            className={am('lb-tool')}
                            href={image.download}
                            download={image.name}
                            title={formatMessage({id: 'fusion.lightbox.download', defaultMessage: 'Download'})}
                            aria-label={formatMessage({id: 'fusion.lightbox.download', defaultMessage: 'Download'})}
                        >
                            <Icon
                                name='upload'
                                size='sm'
                                className={am('flip')}
                            />
                        </a>
                    )}
                    {image.onPreview && tool(formatMessage({id: 'fusion.lightbox.preview', defaultMessage: 'Open in the file preview'}), 'popout', () => {
                        onClose();
                        image.onPreview!();
                    })}
                    {tool(formatMessage({id: 'fusion.lightbox.close', defaultMessage: 'Close image'}), 'x', onClose)}
                </div>
                <div
                    className={am('lb-foot')}
                    onClick={(e) => e.stopPropagation()}
                >
                    {imageCaption(image)}
                    {many && <em>{` · ${index + 1} / ${images.length}`}</em>}
                </div>
                {many && (
                    <>
                        <button
                            type='button'
                            className={am('lb-nav', 'prev')}
                            aria-label={formatMessage({id: 'fusion.lightbox.previous', defaultMessage: 'Previous image'})}
                            onClick={(e) => {
                                e.stopPropagation();
                                go(-1);
                            }}
                        >
                            <Icon name='chev'/>
                        </button>
                        <button
                            type='button'
                            className={am('lb-nav', 'next')}
                            aria-label={formatMessage({id: 'fusion.lightbox.next', defaultMessage: 'Next image'})}
                            onClick={(e) => {
                                e.stopPropagation();
                                go(1);
                            }}
                        >
                            <Icon name='chev'/>
                        </button>
                    </>
                )}
            </div>
        </Layer>
    );
}

// useLightbox gives a component the lightbox it opens, and the function that opens it: on one image, or on one of
// several to go through.
export function useLightbox(): [React.ReactNode, (image: LightboxImage, gallery?: LightboxImage[]) => void] {
    const [open, setOpen] = useState<{images: LightboxImage[]; start: number} | null>(null);
    const close = useCallback(() => setOpen(null), []);
    const show = useCallback((image: LightboxImage, gallery?: LightboxImage[]) => {
        const images = gallery?.length ? gallery : [image];
        setOpen({images, start: Math.max(0, images.findIndex((i) => i.src === image.src))});
    }, []);
    const node = open ? (
        <Lightbox
            images={open.images}
            start={open.start}
            onClose={close}
        />
    ) : null;
    return [node, show];
}
