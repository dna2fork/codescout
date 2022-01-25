import React, { useCallback } from 'react'

interface TrackAnchorClickProps {
    onClick: (event: React.MouseEvent) => void
}

/**
 * Track all anchor link clicks in children components
 */
export const TrackAnchorClick: React.FunctionComponent<TrackAnchorClickProps> = ({ children, onClick }) => {
    const handleClick = useCallback(
        (event: React.MouseEvent<HTMLDivElement, MouseEvent>) => {
            if ((event.target as HTMLElement)?.tagName.toLowerCase() === 'a') {
                onClick(event)
            }
        },
        [onClick]
    )
    return (
        // eslint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/no-static-element-interactions
        <div onClick={handleClick}>{children}</div>
    )
}
