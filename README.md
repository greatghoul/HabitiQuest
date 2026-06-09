# HabitiQuest - Vercel + Go + Gin + Habitica

[![Deploy with Vercel](https://vercel.com/button)](https://vercel.com/new/clone?repository-url=https%3A%2F%2Fgithub.com%2Fgreatghoul%2FHabitiQuest&env=HABITICA_USER_ID,HABITICA_API_TOKEN&envDescription=Habitica%20API%20credentials%20-%20get%20them%20from%20https%3A%2F%2Fhabitica.com%2Fuser%2Fsettings%2Fapi&project-name=habiti-quest&repository-name=habiti-quest)

A Hello World app built with [Go](https://go.dev/) and [Gin](https://gin-gonic.com/), deployed on [Vercel](https://vercel.com/), integrating the [Habitica](https://habitica.com/) API.

## Run Locally

```bash
go mod tidy
go run main.go
```

Open [http://localhost:3000](http://localhost:3000).

## Environment Variables

| Variable | Description |
|---|---|
| `HABITICA_USER_ID` | Your Habitica User ID |
| `HABITICA_API_TOKEN` | Your Habitica API Token |

Get them at [Habitica API Settings](https://habitica.com/user/settings/siteData). After deploying, set them in your Vercel project dashboard under **Settings > Environment Variables**. See [Vercel Environment Variables docs](https://vercel.com/docs/projects/environment-variables) for details.

## Deploy to Vercel

### Option 1: Deploy Button

Click the button above, fill in `HABITICA_USER_ID` and `HABITICA_API_TOKEN`.

### Option 2: Vercel CLI

```bash
npm i -g vercel
vercel --prod
```

### Option 3: Git Integration

Push to GitHub and import the repository in Vercel.
