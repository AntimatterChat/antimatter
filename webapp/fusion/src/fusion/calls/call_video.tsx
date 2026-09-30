// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef} from 'react';

import {am} from 'fusion/utils/class_names';

type Props = {
    stream: MediaStream | null;

    // Screens are shown whole; cameras fill their tile.
    fit?: 'cover' | 'contain';

    // Your own camera, shown as in a mirror.
    mirror?: boolean;
};

function videoTrackId(stream: MediaStream | null | undefined): string {
    return stream?.getVideoTracks()[0]?.id || '';
}

// CallVideo plays a call's camera or screen stream. The element keeps playing while the stream object changes but its
// video track doesn't, so that tiles don't flicker as Calls hands out new streams for the same track.
export default function CallVideo({stream, fit = 'cover', mirror = false}: Props) {
    const ref = useRef<HTMLVideoElement>(null);
    const trackId = videoTrackId(stream);

    useEffect(() => {
        const video = ref.current;
        if (!video) {
            return;
        }
        if (!stream || !trackId) {
            video.srcObject = null;
            return;
        }
        if (videoTrackId(video.srcObject as MediaStream | null) !== trackId) {
            video.srcObject = stream;
            video.play().catch(() => {
                // Autoplay of muted video is allowed; a failure here only means the element went away.
            });
        }
    }, [stream, trackId]);

    return (
        <video
            ref={ref}
            className={am('call-video', {'video-contain': fit === 'contain', mirror})}
            autoPlay={true}
            playsInline={true}

            // The call's sound is played by the Calls plugin, not by the video elements.
            muted={true}
        />
    );
}
