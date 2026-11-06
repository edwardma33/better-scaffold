package templates

import (
	"better-scaffold/internal/config"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
)

//go:embed all:base
var source embed.FS

func Files(c config.Config) (map[string]string, error) {
	deps := map[string]string{"next": "16.4.5", "react": "19.3.1", "react-dom": "19.3.1", "better-auth": "1.7.17", "drizzle-orm": "0.45.4", "pg": "8.23.1", "dotenv": "18.0.9", "server-only": "0.0.1", "clsx": "2.1.1", "tailwind-merge": "3.6.0", "lucide-react": "1.68.0"}
	replacements := map[string]string{"__APP_NAME__": c.Name, "__PM__": c.PackageManager}
	for _, key := range []string{"STRIPE_CUSTOMER", "STRIPE_SCHEMA", "PAYMENT_IMPORTS", "PAYMENT_PLUGINS", "PAYMENT_CLIENT_IMPORT", "PAYMENT_CLIENT_PLUGIN", "SOCIAL_CONFIG", "SOCIAL_BUTTONS", "SOCIAL_LOGO_IMPORTS", "SOCIAL_LOGOS", "QUERY_IMPORT", "QUERY_OPEN", "QUERY_CLOSE", "MOTION_IMPORT", "MOTION_OPEN", "MOTION_CLOSE", "ZOD_IMPORT", "ZOD_VALIDATE", "SOCIAL_DOCS", "PAYMENT_DOCS", "AI_DOCS", "OPTIONAL_DOCS"} {
		replacements["__"+key+"__"] = ""
	}
	out := map[string]string{}
	env := ""
	for _, provider := range c.OAuth {
		upper := strings.ToUpper(provider)
		label := "Google"
		if provider == "github" {
			label = "GitHub"
		}
		env += upper + "_CLIENT_ID=\n" + upper + "_CLIENT_SECRET=\n"
		component := "Google"
		if provider == "github" {
			component = "Github"
		}
		replacements["__SOCIAL_LOGO_IMPORTS__"] += fmt.Sprintf("import { %s } from \"@/components/ui/svgs/%s\";\n", component, provider)
		replacements["__SOCIAL_LOGOS__"] += fmt.Sprintf("{provider.id === %q && <%s aria-hidden=\"true\" className=\"size-4 shrink-0\" />}", provider, component)
		replacements["__SOCIAL_CONFIG__"] += fmt.Sprintf("...(process.env.%s_CLIENT_ID && process.env.%s_CLIENT_SECRET ? { %s: { clientId: process.env.%s_CLIENT_ID, clientSecret: process.env.%s_CLIENT_SECRET } } : {}),", upper, upper, provider, upper, upper)
		replacements["__SOCIAL_BUTTONS__"] += fmt.Sprintf("{ id: %q, label: %q, ready: Boolean(process.env.%s_CLIENT_ID && process.env.%s_CLIENT_SECRET) },", provider, label, upper, upper)
		replacements["__SOCIAL_DOCS__"] += fmt.Sprintf("Create a %s OAuth application. Set `%s_CLIENT_ID` and `%s_CLIENT_SECRET` in `.env`. Register `http://localhost:3000/api/auth/callback/%s` as its callback URL. Restart the dev server after changing credentials.\n\n", label, upper, upper, provider)
	}
	if c.Payments == "stripe" {
		deps["@better-auth/stripe"] = "1.6.29"
		deps["stripe"] = "22.0.0"
		replacements["__STRIPE_CUSTOMER__"] = `stripeCustomerId: text("stripe_customer_id"),`
		replacements["__STRIPE_SCHEMA__"] = `export const subscription = pgTable("subscription", { id: text("id").primaryKey(), plan: text("plan").notNull(), referenceId: text("reference_id").notNull(), stripeCustomerId: text("stripe_customer_id"), stripeSubscriptionId: text("stripe_subscription_id"), status: text("status").default("incomplete"), periodStart: timestamp("period_start"), periodEnd: timestamp("period_end"), cancelAtPeriodEnd: boolean("cancel_at_period_end").default(false), cancelAt: timestamp("cancel_at"), canceledAt: timestamp("canceled_at"), endedAt: timestamp("ended_at"), seats: integer("seats"), trialStart: timestamp("trial_start"), trialEnd: timestamp("trial_end"), billingInterval: text("billing_interval"), stripeScheduleId: text("stripe_schedule_id") });`
		replacements["__PAYMENT_IMPORTS__"] = `import { stripe } from "@better-auth/stripe";
import Stripe from "stripe";`
		replacements["__PAYMENT_PLUGINS__"] = `...(process.env.STRIPE_SECRET_KEY && process.env.STRIPE_WEBHOOK_SECRET ? [stripe({ stripeClient: new Stripe(process.env.STRIPE_SECRET_KEY), stripeWebhookSecret: process.env.STRIPE_WEBHOOK_SECRET, createCustomerOnSignUp: false, subscription: { enabled: true, plans: process.env.STRIPE_PRICE_ID ? [{ name: "pro", priceId: process.env.STRIPE_PRICE_ID }] : [] } })] : []),`
		replacements["__PAYMENT_CLIENT_IMPORT__"] = `import { stripeClient } from "@better-auth/stripe/client";`
		replacements["__PAYMENT_CLIENT_PLUGIN__"] = `stripeClient({ subscription: true })`
		env += "STRIPE_SECRET_KEY=\nSTRIPE_WEBHOOK_SECRET=\nSTRIPE_PRICE_ID=\n"
		replacements["__PAYMENT_DOCS__"] = "Use Stripe test-mode keys. Create a recurring price and set STRIPE_PRICE_ID. Forward Stripe webhooks to `/api/auth/stripe/webhook` and set STRIPE_WEBHOOK_SECRET to the signing secret. Subscribe to checkout.session.completed and customer.subscription.created, customer.subscription.updated, and customer.subscription.deleted. The Better Auth plugin verifies signatures and maintains subscriptions. Import authClient and call `authClient.subscription.upgrade({ plan: 'pro', successUrl: '/dashboard', cancelUrl: '/dashboard' })` or `authClient.subscription.billingPortal({ returnUrl: '/dashboard' })`. No billing UI or access policy is generated. The plugin activates only when both secret values exist. Customer creation on signup is disabled to keep email signup independent of payment availability."
	}
	if c.Payments == "polar" {
		deps["@polar-sh/better-auth"] = "2.1.3"
		deps["@polar-sh/sdk"] = "1.0.2"
		deps["zod"] = "4.6.7"
		replacements["__PAYMENT_IMPORTS__"] = `import { polar, checkout, portal, webhooks } from "@polar-sh/better-auth";
import { createPolarCore } from "@polar-sh/sdk/2026-10";`
		replacements["__PAYMENT_PLUGINS__"] = `...(process.env.POLAR_ACCESS_TOKEN && process.env.POLAR_WEBHOOK_SECRET ? [polar({ client: createPolarCore({ accessToken: process.env.POLAR_ACCESS_TOKEN, environment: "sandbox" }), createCustomerOnSignUp: false, use: [checkout({ products: process.env.POLAR_PRODUCT_ID ? [{ productId: process.env.POLAR_PRODUCT_ID, slug: "pro" }] : [], successUrl: "/dashboard", authenticatedUsersOnly: true }), portal(), webhooks({ secret: process.env.POLAR_WEBHOOK_SECRET })] })] : []),`
		replacements["__PAYMENT_CLIENT_IMPORT__"] = `import { polarClient } from "@polar-sh/better-auth/client";`
		replacements["__PAYMENT_CLIENT_PLUGIN__"] = `polarClient()`
		env += "POLAR_ACCESS_TOKEN=\nPOLAR_WEBHOOK_SECRET=\nPOLAR_PRODUCT_ID=\n"
		replacements["__PAYMENT_DOCS__"] = "Create a Polar sandbox organization, access token, and product. Set POLAR_ACCESS_TOKEN, POLAR_WEBHOOK_SECRET, and POLAR_PRODUCT_ID. Register `/api/auth/polar/webhooks` as your webhook endpoint. The plugin verifies webhook signatures. Import authClient and call `authClient.checkout({ slug: 'pro' })` or `authClient.customer.portal()`. No billing UI or entitlement policy is generated. Change the SDK environment explicitly before using production credentials. The plugin activates only when both secret values exist. Automatic customer creation on signup is disabled so payment network failures do not interrupt email signup."
	}
	if c.AI != "none" && c.AI != "" {
		deps["ai"] = "7.0.163"
		constructor := "createOpenAI"
		pkg := "@ai-sdk/openai"
		key := "OPENAI_API_KEY"
		version := "4.0.109"
		if c.AI == "anthropic" {
			constructor = "createAnthropic"
			pkg = "@ai-sdk/anthropic"
			key = "ANTHROPIC_API_KEY"
			version = "4.0.90"
		}
		deps[pkg] = version
		out["src/lib/ai.ts"] = fmt.Sprintf("import \"server-only\";\nimport { %s } from %q;\nexport function getModel(modelId: string) {\n const apiKey = process.env.%s;\n if (!apiKey) throw new Error(%q);\n return %s({ apiKey })(modelId);\n}\n", constructor, pkg, key, "Set "+key+" in .env before calling getModel.", constructor)
		env += key + "=\n"
		replacements["__AI_DOCS__"] = "Import `getModel` from `@/lib/ai` in server code and pass a provider model ID. Use its result as the model for AI SDK `generateText` or `streamText`. Set " + key + " first. No provider requests run during scaffolding."
	}
	if c.Motion {
		deps["motion"] = "13.4.0"
		out["src/components/fade-in.tsx"] = `"use client";
import { motion } from "motion/react";
export function FadeIn({ children }: { children: React.ReactNode }) { return <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="space-y-6">{children}</motion.div>; }
`
		replacements["__MOTION_IMPORT__"] = `import { FadeIn } from "@/components/fade-in";`
		replacements["__MOTION_OPEN__"] = "<FadeIn>"
		replacements["__MOTION_CLOSE__"] = "</FadeIn>"
	}
	if c.Query {
		deps["@tanstack/react-query"] = "5.102.8"
		out["src/components/query-provider.tsx"] = `"use client";
import { useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
export function QueryProvider({ children }: { children: React.ReactNode }) { const [client] = useState(() => new QueryClient()); return <QueryClientProvider client={client}>{children}</QueryClientProvider>; }
`
		out["src/lib/query-example.ts"] = `"use client";
import { useQuery } from "@tanstack/react-query";
export function useExample() { return useQuery({ queryKey: ["example"], queryFn: async () => ({ message: "Query is ready" }) }); }
`
		replacements["__QUERY_IMPORT__"] = `import { QueryProvider } from "@/components/query-provider";`
		replacements["__QUERY_OPEN__"] = "<QueryProvider>"
		replacements["__QUERY_CLOSE__"] = "</QueryProvider>"
		replacements["__OPTIONAL_DOCS__"] += "TanStack Query is mounted in the root layout. Import useExample from `src/lib/query-example.ts` in a client component.\n\n"
	}
	if c.Zod {
		deps["zod"] = "4.6.7"
		replacements["__ZOD_IMPORT__"] = `import { z } from "zod";`
		replacements["__ZOD_VALIDATE__"] = `const parsed = z.object({ email: z.email(), password: z.string().min(8), name: mode === "sign-up" ? z.string().min(1) : z.string() }).safeParse({ email, password, name }); if (!parsed.success) return { error: parsed.error.issues[0]?.message || "Invalid input" };`
		replacements["__OPTIONAL_DOCS__"] += "Zod validates auth form input in the server action.\n\n"
	}
	if c.Elysia {
		deps["elysia"] = "1.4.30"
		deps["@elysia/eden"] = "1.4.10"
		out["src/server/api.ts"] = `import { Elysia } from "elysia";
export const api = new Elysia({ prefix: "/api/rpc" }).get("/health", () => ({ ok: true }));
export type App = typeof api;
`
		out["src/app/api/rpc/[[...slugs]]/route.ts"] = `import { api } from "@/server/api";
export const GET = api.fetch;
`
		out["src/lib/eden.ts"] = `"use client";
import { treaty } from "@elysia/eden";
import type { App } from "@/server/api";
export function getAPI() { return treaty<App>(window.location.origin); }
`
		replacements["__OPTIONAL_DOCS__"] += "Elysia serves GET /api/rpc/health through Next.js. In browser code call `getAPI().api.rpc.health.get()` using `@/lib/eden`. The server implementation is never imported into the browser client.\n\n"
	}
	encoded, err := json.Marshal(deps)
	if err != nil {
		return nil, err
	}
	replacements["__DEPENDENCIES__"] = string(encoded)
	err = fs.WalkDir(source, "base", func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		data, e := source.ReadFile(path)
		if e != nil {
			return e
		}
		body := string(data)
		for a, b := range replacements {
			body = strings.ReplaceAll(body, a, b)
		}
		out[strings.TrimSuffix(strings.TrimPrefix(path, "base/"), ".tmpl")] = body
		return nil
	})
	if err != nil {
		return nil, err
	}
	out[".env.example"] += env
	return out, nil
}
