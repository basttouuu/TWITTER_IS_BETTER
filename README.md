# Twitter is Better

Application web Go de micro-publications, déployable sur Scalingo. Les comptes,
les sessions et les publications sont stockés dans MySQL.

## Lancer en local

1. Installer Go 1.25+ et démarrer un serveur MySQL.
2. Définir l'URL MySQL dans le terminal, par exemple :

```powershell
$env:SCALINGO_MYSQL_URL = "mysql://user:password@localhost:3306/twitter?ssl-mode=DISABLED"
```

3. Lancer l'application :

```powershell
go mod download
go run .
```

Le serveur écoute sur `http://localhost:8080` (ou sur le port défini par
`PORT`). Les tables `users`, `sessions` et `posts` sont créées au démarrage.

## Déployer sur Scalingo

1. Créer une application Scalingo à partir de ce dépôt et y ajouter un addon
   MySQL.
2. Vérifier que l'addon expose son URL sous `SCALINGO_MYSQL_URL`. L'application
   accepte aussi `MYSQL_URL` ou `DATABASE_URL` pour les autres environnements.
3. Définir `APP_ENV=production` dans les variables d'environnement Scalingo afin
   d'activer les cookies sécurisés. `PORT` est fourni par la plateforme.
4. Déployer l'application. Le `Procfile` démarre le serveur et les tables sont
   initialisées automatiquement.

Les mots de passe utilisateurs sont hachés avec bcrypt. Ne commitez jamais
votre fichier `.env` ni les identifiants de connexion à la base.