package commands

import "github.com/disgoorg/disgo/discord"

var Commands = []discord.ApplicationCommandCreate{
	version,
	create,
	remove,
	flag,
	deleteFlag,
	report,
	creds,
	deleteCreds,
	nextCTFs,
	cinit,
	chall,
	vote,
	ping,
	help,
	guide,
	rules,
}
