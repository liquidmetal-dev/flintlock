import React from 'react';
import clsx from 'clsx';
import styles from './HomepageFeatures.module.css';

const FeatureList = [
  {
    title: 'Flintlock',
    Svg: require('../../static/img/logo.svg').default,
    description: (
      <>
        A streamlined service to manage the lifecycle of microVMs.
        Flintlock lets you focus on deploying your application in MicroVMs
        tailored for its need.
      </>
    ),
  },
  {
    title: 'Backed by Container Images',
    Svg: require('../../static/img/containerdio-icon.svg').default,
    description: (
      <>
        With flintlock, you can use OCI images to supply kernel binaries
        and Operating Systems to your VMs; no more large and cumbersome
        filesystem images.
      </>
    ),
  },
];

function Feature({Svg, title, description}) {
  return (
    <div className={clsx('col col--6')}>
      <div className="text--center">
        <Svg className={styles.featureSvg} alt={title} />
      </div>
      <div className="text--center padding-horiz--md">
        <h3>{title}</h3>
        <p>{description}</p>
      </div>
    </div>
  );
}

export default function HomepageFeatures() {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}
