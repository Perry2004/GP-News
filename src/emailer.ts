import {
	SESClient,
	SendEmailCommand,
	type SendEmailCommandOutput,
} from "@aws-sdk/client-ses";
import z from "zod";

const envVars = z
	.object({
		EMAIL_RECEIVERS: z
			.string()
			.min(1)
			.transform((str) => {
				const splitted = str.split(",");
				return splitted.map((s) => s.trim()).filter((s) => s.length > 0);
			}),
		EMAIL_SENDER: z.string().min(1),
		AWS_REGION: z.string().min(1),
	})
	.parse(process.env);

const EMAIL_RECEIVERS = envVars.EMAIL_RECEIVERS;
const EMAIL_SENDER = envVars.EMAIL_SENDER;
const SUBJECT = "GP-News Briefing";
const SES_REGION = envVars.AWS_REGION;

const sesClient = new SESClient({
	region: SES_REGION,
});

export async function sendBriefingEmail(
	briefing: string,
	subject: string = SUBJECT,
): Promise<SendEmailCommandOutput[]> {
	if (EMAIL_RECEIVERS.length === 0) {
		throw new Error("EMAIL_RECEIVERS is empty after parsing");
	}

	return Promise.all(
		EMAIL_RECEIVERS.map((receiver) => {
			return sesClient.send(
				new SendEmailCommand({
					Source: EMAIL_SENDER,
					Destination: {
						ToAddresses: [receiver],
					},
					Message: {
						Subject: {
							Data: subject,
							Charset: "UTF-8",
						},
						Body: {
							Text: {
								Data: briefing,
								Charset: "UTF-8",
							},
						},
					},
				}),
			);
		}),
	);
}
