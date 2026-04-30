package ya

type AudioFormat string

const (
	AudioFormatMP3  AudioFormat = "mp3"
	AudioFormatFLAC AudioFormat = "flac"
)

type DownloadOptions struct {
	SkipCover   bool
	AudioFormat AudioFormat
}

func (o DownloadOptions) FormatOrDefault() AudioFormat {
	if o.AudioFormat == AudioFormatFLAC {
		return AudioFormatFLAC
	}
	return AudioFormatMP3
}
