import React, { useRef, useState } from 'react'
import { useHistory } from 'react-router'

import { Button, useAutoFocus, Modal, Link } from '@sourcegraph/wildcard'

import { FourLineChart, LangStatsInsightChart, ThreeLineChart } from './components/MediaCharts'
import styles from './GaConfirmationModal.module.scss'

export const GaConfirmationModal: React.FunctionComponent = () => {
    const history = useHistory()
    const [isGaAccepted, setGaAccepted] = useState(false)

    // You are on Sourcegraph 3.37.x
    const isSourcegraph3_37_x = false

    // It is after Feb 24, 2022
    const feb_24_2022 = 1645660800000
    const isAfterFeb_24_2022 = Date.now() > feb_24_2022
    // You have not already accepted it
    const isNotAlreadyAccepted = !isGaAccepted

    // You haven't purchased it (no code-insights license tag)
    const isNotPurchased = false

    const showConfirmationModal = isSourcegraph3_37_x && isAfterFeb_24_2022 && isNotAlreadyAccepted && isNotPurchased

    if (!showConfirmationModal) {
        return null
    }

    const handleAccept = (): void => {
        setGaAccepted(true)
    }

    const handleDismiss = (): void => {
        history.push('/')
    }

    return (
        <Modal position="center" aria-label="Code Insights Ga information" containerClassName={styles.overlay}>
            <GaConfirmationModalContent onAccept={handleAccept} onDismiss={handleDismiss} />
        </Modal>
    )
}

interface GaConfirmationModalContentProps {
    onAccept: () => void
    onDismiss: () => void
}

/**
 * Renders Code Insights Ga modal content component.
 * Exported especially for storybook story component cause chromatic has a problem of rendering modals
 * on CI.
 */
export const GaConfirmationModalContent: React.FunctionComponent<GaConfirmationModalContentProps> = props => {
    const { onAccept, onDismiss } = props
    const dismissButtonReference = useRef<HTMLButtonElement>(null)

    useAutoFocus({ autoFocus: true, reference: dismissButtonReference })

    return (
        <>
            <h1 className={styles.title}>Welcome to the Code Insights Ga!</h1>

            <div className={styles.mediaHeroContent}>
                <ThreeLineChart className={styles.chart} />
                <FourLineChart className={styles.chart} />
                <LangStatsInsightChart className={styles.chart} />
            </div>

            <div className={styles.textContent}>
                <p>
                    <b>🥁 We’re introducing Code Insights</b>: a new analytics tool that lets you track and understand
                    what’s in your code and how it changes <b>over time</b>.
                </p>

                <p>
                    Track anything that can be expressed with a Sourcegraph search query: migrations, package use,
                    version adoption, code smells, codebase size, and more, across 1,000s of repositories.
                </p>

                <p>
                    We're still polishing Code Insights and you might find bugs while we’re in ga. Please{' '}
                    <Link to="/help/code_insights#code-insights-ga" target="_blank" rel="noopener">
                        share any bugs 🐛 or feedback
                    </Link>{' '}
                    to help us make Code Insights better.
                </p>

                <p>
                    Code Insights is <b>free while in ga through 2021</b>. When Code Insights is officially released, we
                    may disable your use of the product or charge for continued use.
                </p>
            </div>

            <footer className={styles.actions}>
                <Button ref={dismissButtonReference} variant="secondary" outline={true} onClick={onDismiss}>
                    Maybe later
                </Button>

                <Button variant="primary" onClick={onAccept}>
                    Understood, let’s go!
                </Button>
            </footer>
        </>
    )
}
