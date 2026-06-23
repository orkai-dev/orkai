export type ContrastColumn = {
  heading: string;
  tone: 'them' | 'us';
  points: string[];
};

export type LedgerRow = {
  label: string;
  title: string;
  items: string[];
};

export type HomepageContent = {
  meta: {
    title: string;
    description: string;
  };
  copy: {
    copied: string;
    copy: string;
    copyCommand: string;
  };
  hero: {
    eyebrow: string;
    titleLine1: string;
    titleLine2: string;
    lead: string;
    ctaPrimary: string;
    ctaSecondary: string;
    proof: string[];
    terminalReady: string;
    terminalCreate: string;
  };
  valueProps: Array<{icon: string; title: string; body: string}>;
  why: {
    eyebrow: string;
    title: string;
    lead: string;
    contrast: ContrastColumn[];
  };
  resourceMap: {
    eyebrow: string;
    title: string;
    lead: string;
    footnote: string;
    tableHeadResource: string;
    tableHeadStatus: string;
  };
  cluster: {
    eyebrow: string;
    title: string;
    lead: string;
    list: string[];
  };
  pages: {
    eyebrow: string;
    title: string;
    lead: string;
    list: string[];
    implLabel: string;
    implPath: string;
  };
  ledger: {
    eyebrow: string;
    title: string;
    rows: LedgerRow[];
  };
  cta: {
    eyebrow: string;
    title: string;
    lead: string;
    ctaPrimary: string;
    ctaSecondary: string;
    installLabel: string;
    github: string;
  };
};

const en: HomepageContent = {
  meta: {
    title: 'Self-hosted PaaS on Kubernetes',
    description:
      'Self-hosted PaaS on K3s. Git-push and image deploys where every app, database, and cron job is a real Kubernetes object you can still drive with kubectl and helm.',
  },
  copy: {
    copied: 'Copied',
    copy: 'Copy',
    copyCommand: 'command',
  },
  hero: {
    eyebrow: '// self-hosted paas · k3s-native',
    titleLine1: 'Self-hosted PaaS.',
    titleLine2: 'Real Kubernetes underneath.',
    lead:
      "Orka'i runs git-push and container-image deploys on K3s you control. Every app, database, and cron job is a native Kubernetes object — so kubectl and helm keep working, and nothing hides the cluster from you.",
    ctaPrimary: "Self-host orka'i",
    ctaSecondary: 'View source',
    proof: ['K3s-native', 'Kaniko builds', 'kubectl / helm', 'RBAC + 2FA'],
    terminalReady: 'ready',
    terminalCreate: "orka'i app create core-api --from-git",
  },
  valueProps: [
    {
      icon: 'rocket_launch',
      title: 'Git-push deploys',
      body: 'Push a branch and Kaniko builds the image inside the cluster. No Docker socket, no external CI.',
    },
    {
      icon: 'database',
      title: 'Managed resources',
      body: 'Postgres, Valkey, and CronJobs provisioned as real StatefulSets, Services, and Jobs.',
    },
    {
      icon: 'terminal',
      title: 'kubectl-native',
      body: "Orka'i never takes the kubeconfig away. helm, k9s, and your GitOps tooling keep working.",
    },
  ],
  why: {
    eyebrow: "// why orka'i",
    title: 'Not a wrapper. The control plane.',
    lead:
      "Most self-hosted PaaS tools wrap Docker and put a UI on top. Orka'i adds a thin, opinionated layer for builds, routing, backups, and secrets — and leaves the cluster fully exposed underneath.",
    contrast: [
      {
        heading: 'Docker-wrapper PaaS',
        tone: 'them',
        points: [
          'Workloads live inside the tool’s own abstraction over the Docker socket.',
          'kubectl shows the tool’s containers, not a topology you control.',
          'Outgrowing it means a migration off the platform.',
        ],
      },
      {
        heading: "Orka'i on K3s",
        tone: 'us',
        points: [
          'Workloads are real Deployments, StatefulSets, CronJobs, and Services.',
          'kubectl and helm operate directly against the same objects.',
          'Leave whenever you want — underneath, it was just Kubernetes.',
        ],
      },
    ],
  },
  resourceMap: {
    eyebrow: '// resource map',
    title: 'One deploy. Real objects.',
    lead:
      "orka'i deploy core-api resolves to native Kubernetes resources you can inspect, patch, and audit with the tools you already run.",
    footnote: 'Output equivalent to kubectl get all -n production.',
    tableHeadResource: 'resource',
    tableHeadStatus: 'status',
  },
  cluster: {
    eyebrow: '// direct access',
    title: 'Your tools still reach the cluster.',
    lead:
      'Inspect nodes, list Helm releases, watch DaemonSets, and read Traefik routes with the tools you already run. Provision a managed database and it lands as a StatefulSet and a PVC — nothing proprietary in the way.',
    list: [
      'Cluster inspection: nodes, Helm, DaemonSets, Traefik.',
      'Namespaces-as-environments with RBAC and required 2FA.',
      'Audit everything with kubectl, helm, or your GitOps tool.',
    ],
  },
  pages: {
    eyebrow: '// pages',
    title: 'Static sites without the cluster.',
    lead:
      'Pages sync a publish folder from git to your own S3 bucket and CloudFront distribution — clone, mirror, invalidate. No Kubernetes object and no in-platform build step in MVP; just ready-to-serve files.',
    list: [
      'Repo + branch + publish folder (e.g. dist/, docs/, or repo root)',
      'Lazy-provisioned S3, Origin Access Control, and CloudFront in your AWS account',
      'Deploy from the control plane; live URL when the CDN clears',
    ],
    implLabel: 'Implementation reference',
    implPath: '.spec/impl/pages.md',
  },
  ledger: {
    eyebrow: '// capabilities',
    title: 'What you deploy and manage.',
    rows: [
      {
        label: 'deploy',
        title: 'Ship from git or an image',
        items: [
          'Git-push deploys with in-cluster Kaniko builds',
          'Container-image deploys for anything already built',
          'Pages: static sites from git to S3 + CloudFront',
          'Rolling updates with health-probe cutover and automatic rollback',
        ],
      },
      {
        label: 'run',
        title: 'Managed resources, native objects',
        items: [
          'Managed Postgres and Valkey with PVC binding and credential rotation',
          'Kubernetes CronJobs for scheduled work and backups',
          'Traefik ingress, custom domains, and Let’s Encrypt TLS on demand',
        ],
      },
      {
        label: 'operate',
        title: 'See and control the whole cluster',
        items: [
          'Inspect nodes, Helm releases, DaemonSets, and Traefik routing',
          'RBAC with namespaces-as-environments and required 2FA',
          'Multi-channel notifications wired to deploy and health events',
        ],
      },
    ],
  },
  cta: {
    eyebrow: '// get started',
    title: 'Run it on hardware you own.',
    lead:
      'Open source and in active development. Install the control plane on a fresh Linux box or an existing K3s cluster.',
    ctaPrimary: "Self-host orka'i",
    ctaSecondary: 'Read the docs',
    installLabel: 'one-line install',
    github: 'Star on GitHub',
  },
};

const es: HomepageContent = {
  meta: {
    title: 'PaaS autoalojado en Kubernetes',
    description:
      'PaaS autoalojado en K3s. Despliegues por git-push e imagen donde cada app, base de datos y cron job es un objeto Kubernetes real que sigues controlando con kubectl y helm.',
  },
  copy: {
    copied: 'Copiado',
    copy: 'Copiar',
    copyCommand: 'comando',
  },
  hero: {
    eyebrow: '// paas autoalojado · nativo en k3s',
    titleLine1: 'PaaS autoalojado.',
    titleLine2: 'Kubernetes real debajo.',
    lead:
      "Orka'i ejecuta despliegues por git-push e imagen de contenedor en K3s que tú controlas. Cada app, base de datos y cron job es un objeto Kubernetes nativo — kubectl y helm siguen funcionando y nada oculta el clúster.",
    ctaPrimary: "Autoaloja orka'i",
    ctaSecondary: 'Ver código',
    proof: ['Nativo en K3s', 'Builds con Kaniko', 'kubectl / helm', 'RBAC + 2FA'],
    terminalReady: 'listo',
    terminalCreate: "orka'i app create core-api --from-git",
  },
  valueProps: [
    {
      icon: 'rocket_launch',
      title: 'Despliegues git-push',
      body: 'Haz push de una rama y Kaniko construye la imagen dentro del clúster. Sin socket Docker ni CI externo.',
    },
    {
      icon: 'database',
      title: 'Recursos gestionados',
      body: 'Postgres, Valkey y CronJobs aprovisionados como StatefulSets, Services y Jobs reales.',
    },
    {
      icon: 'terminal',
      title: 'Nativo en kubectl',
      body: "Orka'i nunca te quita el kubeconfig. helm, k9s y tu tooling GitOps siguen funcionando.",
    },
  ],
  why: {
    eyebrow: "// por qué orka'i",
    title: 'No es un wrapper. Es el plano de control.',
    lead:
      "La mayoría de PaaS autoalojados envuelven Docker y ponen una UI encima. Orka'i añade una capa fina y opinada para builds, enrutamiento, backups y secretos — y deja el clúster totalmente expuesto debajo.",
    contrast: [
      {
        heading: 'PaaS wrapper de Docker',
        tone: 'them',
        points: [
          'Las cargas viven dentro de la abstracción propia de la herramienta sobre el socket Docker.',
          'kubectl muestra los contenedores de la herramienta, no una topología que controles.',
          'Superarlo implica migrar fuera de la plataforma.',
        ],
      },
      {
        heading: "Orka'i en K3s",
        tone: 'us',
        points: [
          'Las cargas son Deployments, StatefulSets, CronJobs y Services reales.',
          'kubectl y helm operan directamente sobre los mismos objetos.',
          'Sal cuando quieras — debajo, siempre fue Kubernetes.',
        ],
      },
    ],
  },
  resourceMap: {
    eyebrow: '// mapa de recursos',
    title: 'Un despliegue. Objetos reales.',
    lead:
      "orka'i deploy core-api se resuelve en recursos Kubernetes nativos que puedes inspeccionar, parchear y auditar con las herramientas que ya usas.",
    footnote: 'Salida equivalente a kubectl get all -n production.',
    tableHeadResource: 'recurso',
    tableHeadStatus: 'estado',
  },
  cluster: {
    eyebrow: '// acceso directo',
    title: 'Tus herramientas siguen llegando al clúster.',
    lead:
      'Inspecciona nodos, lista releases de Helm, observa DaemonSets y lee rutas Traefik con las herramientas que ya usas. Aprovisiona una base de datos gestionada y aterriza como StatefulSet y PVC — nada propietario en el camino.',
    list: [
      'Inspección del clúster: nodos, Helm, DaemonSets, Traefik.',
      'Namespaces como entornos con RBAC y 2FA obligatorio.',
      'Audita todo con kubectl, helm o tu herramienta GitOps.',
    ],
  },
  pages: {
    eyebrow: '// pages',
    title: 'Sitios estáticos sin el clúster.',
    lead:
      'Pages sincroniza una carpeta de publicación desde git a tu bucket S3 y distribución CloudFront — clonar, reflejar, invalidar. Sin objeto Kubernetes ni paso de build en la plataforma en el MVP; solo archivos listos para servir.',
    list: [
      'Repo + rama + carpeta de publicación (p. ej. dist/, docs/ o la raíz)',
      'S3, Origin Access Control y CloudFront aprovisionados bajo demanda en tu cuenta AWS',
      'Despliega desde el plano de control; URL en vivo cuando el CDN se actualiza',
    ],
    implLabel: 'Referencia de implementación',
    implPath: '.spec/impl/pages.md',
  },
  ledger: {
    eyebrow: '// capacidades',
    title: 'Lo que despliegas y gestionas.',
    rows: [
      {
        label: 'deploy',
        title: 'Publica desde git o una imagen',
        items: [
          'Despliegues git-push con builds Kaniko en el clúster',
          'Despliegues por imagen de contenedor para lo ya construido',
          'Pages: sitios estáticos de git a S3 + CloudFront',
          'Actualizaciones rolling con health probes y rollback automático',
        ],
      },
      {
        label: 'run',
        title: 'Recursos gestionados, objetos nativos',
        items: [
          'Postgres y Valkey gestionados con PVC y rotación de credenciales',
          'CronJobs de Kubernetes para tareas programadas y backups',
          'Ingress Traefik, dominios personalizados y TLS Let’s Encrypt bajo demanda',
        ],
      },
      {
        label: 'operate',
        title: 'Ve y controla todo el clúster',
        items: [
          'Inspecciona nodos, releases Helm, DaemonSets y enrutamiento Traefik',
          'RBAC con namespaces como entornos y 2FA obligatorio',
          'Notificaciones multicanal conectadas a eventos de despliegue y salud',
        ],
      },
    ],
  },
  cta: {
    eyebrow: '// empezar',
    title: 'Ejecútalo en hardware que posees.',
    lead:
      'Código abierto y en desarrollo activo. Instala el plano de control en un Linux nuevo o en un clúster K3s existente.',
    ctaPrimary: "Autoaloja orka'i",
    ctaSecondary: 'Leer la documentación',
    installLabel: 'instalación en una línea',
    github: 'Dar estrella en GitHub',
  },
};

const pt: HomepageContent = {
  meta: {
    title: 'PaaS self-hosted em Kubernetes',
    description:
      'PaaS self-hosted em K3s. Deploys por git-push e imagem onde cada app, banco de dados e cron job é um objeto Kubernetes real que você ainda controla com kubectl e helm.',
  },
  copy: {
    copied: 'Copiado',
    copy: 'Copiar',
    copyCommand: 'comando',
  },
  hero: {
    eyebrow: '// paas self-hosted · nativo em k3s',
    titleLine1: 'PaaS self-hosted.',
    titleLine2: 'Kubernetes de verdade por baixo.',
    lead:
      "Orka'i executa deploys por git-push e imagem de contêiner no K3s que você controla. Cada app, banco de dados e cron job é um objeto Kubernetes nativo — kubectl e helm continuam funcionando e nada esconde o cluster.",
    ctaPrimary: "Self-host orka'i",
    ctaSecondary: 'Ver código',
    proof: ['Nativo em K3s', 'Builds com Kaniko', 'kubectl / helm', 'RBAC + 2FA'],
    terminalReady: 'pronto',
    terminalCreate: "orka'i app create core-api --from-git",
  },
  valueProps: [
    {
      icon: 'rocket_launch',
      title: 'Deploys git-push',
      body: 'Faça push de um branch e o Kaniko constrói a imagem dentro do cluster. Sem socket Docker, sem CI externo.',
    },
    {
      icon: 'database',
      title: 'Recursos gerenciados',
      body: 'Postgres, Valkey e CronJobs provisionados como StatefulSets, Services e Jobs reais.',
    },
    {
      icon: 'terminal',
      title: 'Nativo em kubectl',
      body: "Orka'i nunca tira o kubeconfig. helm, k9s e seu tooling GitOps continuam funcionando.",
    },
  ],
  why: {
    eyebrow: "// por que orka'i",
    title: 'Não é um wrapper. É o plano de controle.',
    lead:
      "A maioria dos PaaS self-hosted envolve Docker e coloca uma UI por cima. Orka'i adiciona uma camada fina e opinativa para builds, roteamento, backups e secrets — e deixa o cluster totalmente exposto por baixo.",
    contrast: [
      {
        heading: 'PaaS wrapper de Docker',
        tone: 'them',
        points: [
          'Cargas vivem dentro da abstração da ferramenta sobre o socket Docker.',
          'kubectl mostra os contêineres da ferramenta, não uma topologia que você controla.',
          'Crescer além disso significa migrar para fora da plataforma.',
        ],
      },
      {
        heading: "Orka'i no K3s",
        tone: 'us',
        points: [
          'Cargas são Deployments, StatefulSets, CronJobs e Services reais.',
          'kubectl e helm operam diretamente nos mesmos objetos.',
          'Saia quando quiser — por baixo, sempre foi Kubernetes.',
        ],
      },
    ],
  },
  resourceMap: {
    eyebrow: '// mapa de recursos',
    title: 'Um deploy. Objetos reais.',
    lead:
      "orka'i deploy core-api resolve em recursos Kubernetes nativos que você pode inspecionar, patchar e auditar com as ferramentas que já usa.",
    footnote: 'Saída equivalente a kubectl get all -n production.',
    tableHeadResource: 'recurso',
    tableHeadStatus: 'status',
  },
  cluster: {
    eyebrow: '// acesso direto',
    title: 'Suas ferramentas ainda alcançam o cluster.',
    lead:
      'Inspecione nós, liste releases Helm, observe DaemonSets e leia rotas Traefik com as ferramentas que já usa. Provisione um banco gerenciado e ele vira StatefulSet e PVC — nada proprietário no caminho.',
    list: [
      'Inspeção do cluster: nós, Helm, DaemonSets, Traefik.',
      'Namespaces como ambientes com RBAC e 2FA obrigatório.',
      'Audite tudo com kubectl, helm ou sua ferramenta GitOps.',
    ],
  },
  pages: {
    eyebrow: '// pages',
    title: 'Sites estáticos sem o cluster.',
    lead:
      'Pages sincroniza uma pasta de publicação do git para seu bucket S3 e distribuição CloudFront — clone, espelha, invalida. Sem objeto Kubernetes e sem etapa de build na plataforma no MVP; apenas arquivos prontos para servir.',
    list: [
      'Repo + branch + pasta de publicação (ex.: dist/, docs/ ou raiz do repo)',
      'S3, Origin Access Control e CloudFront provisionados sob demanda na sua conta AWS',
      'Deploy pelo plano de controle; URL ao vivo quando o CDN atualiza',
    ],
    implLabel: 'Referência de implementação',
    implPath: '.spec/impl/pages.md',
  },
  ledger: {
    eyebrow: '// capacidades',
    title: 'O que você faz deploy e gerencia.',
    rows: [
      {
        label: 'deploy',
        title: 'Publique a partir de git ou imagem',
        items: [
          'Deploys git-push com builds Kaniko no cluster',
          'Deploys por imagem de contêiner para o que já foi construído',
          'Pages: sites estáticos de git para S3 + CloudFront',
          'Rolling updates com health probes e rollback automático',
        ],
      },
      {
        label: 'run',
        title: 'Recursos gerenciados, objetos nativos',
        items: [
          'Postgres e Valkey gerenciados com PVC e rotação de credenciais',
          'CronJobs Kubernetes para trabalho agendado e backups',
          'Ingress Traefik, domínios customizados e TLS Let’s Encrypt sob demanda',
        ],
      },
      {
        label: 'operate',
        title: 'Veja e controle todo o cluster',
        items: [
          'Inspecione nós, releases Helm, DaemonSets e roteamento Traefik',
          'RBAC com namespaces como ambientes e 2FA obrigatório',
          'Notificações multicanal ligadas a eventos de deploy e saúde',
        ],
      },
    ],
  },
  cta: {
    eyebrow: '// começar',
    title: 'Rode no hardware que você possui.',
    lead:
      'Código aberto e em desenvolvimento ativo. Instale o plano de controle em um Linux novo ou em um cluster K3s existente.',
    ctaPrimary: "Self-host orka'i",
    ctaSecondary: 'Ler a documentação',
    installLabel: 'instalação em uma linha',
    github: 'Dar estrela no GitHub',
  },
};

const contentByLocale: Record<string, HomepageContent> = {en, es, pt};

export function getHomepageContent(locale: string): HomepageContent {
  return contentByLocale[locale] ?? en;
}
