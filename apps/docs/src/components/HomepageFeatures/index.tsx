import type {ReactNode} from 'react';
import clsx from 'clsx';
import Heading from '@theme/Heading';
import styles from './styles.module.css';

type FeatureItem = {
  title: string;
  tag: string;
  description: ReactNode;
};

const FeatureList: FeatureItem[] = [
  {
    title: 'Real Kubernetes',
    tag: 'runtime:k3s',
    description: (
      <>
        Every workload is a native K8s object — Deployments, StatefulSets, CronJobs.
        Use <code>kubectl</code> and <code>helm</code> directly against your cluster.
      </>
    ),
  },
  {
    title: 'Deploy Anything',
    tag: 'deploy:git|image',
    description: (
      <>
        Git-push deploys with in-cluster Kaniko builds, or pull pre-built Docker images.
        Rolling updates, health probes, and autoscaling included.
      </>
    ),
  },
  {
    title: 'Managed Services',
    tag: 'svc:db+cron',
    description: (
      <>
        Provision databases, schedule cron jobs, and inspect ingress — all from one
        control plane with live logs and cluster topology.
      </>
    ),
  },
];

function Feature({title, tag, description}: FeatureItem) {
  return (
    <div className={clsx('col col--4')}>
      <div className={styles.featureCard}>
        <span className={styles.featureTag}>{tag}</span>
        <Heading as="h3" className={styles.featureTitle}>
          {title}
        </Heading>
        <p className={styles.featureDescription}>{description}</p>
      </div>
    </div>
  );
}

export default function HomepageFeatures(): ReactNode {
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
